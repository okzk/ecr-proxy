package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/okzk/ticker"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

var service *ecr.ECR
var authorizationData atomic.Value

func init() {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatalf("failed to create a new session.\n %v", err)
	}
	if *sess.Config.Region == "" {
		region, err := ec2metadata.New(sess).Region()
		if err != nil {
			log.Fatalf("could not find region configurations")
		}
		sess.Config.Region = aws.String(region)
	}

	service = ecr.New(sess)
}

func refreshAuthorizationData() {
	r, err := service.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.Fatalf("failed to get auth token.\n %v", err)
	}
	if len(r.AuthorizationData) == 0 {
		log.Fatalf("empty auth data.")
	}
	log.Println("token refreshed.")
	authorizationData.Store(r.AuthorizationData[0])
}

func main() {
	refreshAuthorizationData()
	ticker.New(time.Hour*10, func(_ time.Time) { refreshAuthorizationData() })

	director := func(r *http.Request) {
		token := authorizationData.Load().(*ecr.AuthorizationData)
		u, _ := url.Parse(aws.StringValue(token.ProxyEndpoint))
		r.URL.Scheme = u.Scheme
		r.URL.Host = u.Host
		r.Host = u.Host
		r.Header.Set("Authorization", "Basic "+aws.StringValue(token.AuthorizationToken))
	}
	server := http.Server{
		Addr:    ":5000",
		Handler: &httputil.ReverseProxy{Director: director},
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

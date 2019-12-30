package aws

import "github.com/aws/aws-sdk-go/aws/session"

func CreateSession() *session.Session {
	return session.Must(session.NewSession())
}

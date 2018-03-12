package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/common"
)

func TestUserTransform(t *testing.T) {
	id := "1234"
	ip := "127.0.0.1"
	email := "test@mail.co"
	username := "user123"
	userAgent := "rum-1.0"

	tests := []struct {
		User   User
		Output common.MapStr
	}{
		{
			User:   User{},
			Output: common.MapStr{},
		},
		{
			User: User{
				Id:        &id,
				IP:        &ip,
				Email:     &email,
				Username:  &username,
				UserAgent: &userAgent,
			},
			Output: common.MapStr{
				"ip":         "127.0.0.1",
				"id":         "1234",
				"email":      "test@mail.co",
				"username":   "user123",
				"user_agent": "rum-1.0",
			},
		},
	}

	for _, test := range tests {
		output := test.User.Transform()
		assert.Equal(t, test.Output, output)
	}
}

func TestUserEnrich(t *testing.T) {
	ip := "127.0.0.1"
	userAgent := "ruby-1.0"
	email := "user@email.co"
	for _, test := range []struct {
		user  *User
		input pr.Intake
		out   *User
	}{
		{
			user:  nil,
			input: pr.Intake{},
			out:   nil,
		},
		{
			user:  nil,
			input: pr.Intake{UserIP: ip},
			out:   &User{IP: &ip},
		},
		{
			user:  &User{Email: &email},
			input: pr.Intake{},
			out:   &User{Email: &email},
		},
		{
			user:  &User{Email: &email},
			input: pr.Intake{SystemIP: ip},
			out:   &User{Email: &email},
		},
		{
			user:  &User{Email: &email},
			input: pr.Intake{UserIP: ip},
			out:   &User{Email: &email, IP: &ip},
		},
		{
			user:  &User{Email: &email},
			input: pr.Intake{UserAgent: userAgent},
			out:   &User{Email: &email, UserAgent: &userAgent},
		},
		{
			user:  &User{Email: &email},
			input: pr.Intake{UserIP: ip, UserAgent: userAgent},
			out:   &User{Email: &email, IP: &ip, UserAgent: &userAgent},
		},
	} {
		assert.Equal(t, test.out, test.user.Enrich(test.input))
	}
}

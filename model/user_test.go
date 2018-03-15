package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

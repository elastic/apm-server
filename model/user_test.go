package model

import (
	"errors"
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
				"user-agent": "rum-1.0",
			},
		},
	}

	for _, test := range tests {
		output := test.User.Transform()
		assert.Equal(t, test.Output, output)
	}
}

func TestUserDecode(t *testing.T) {
	id, mail, name, ip, agent := "12", "m@g.dk", "foo", "127.0.0.1", "ruby"
	inpErr := errors.New("some error happened")
	for _, test := range []struct {
		input    interface{}
		inputErr error
		err      error
		u        *User
	}{
		{input: nil, inputErr: nil, err: nil, u: nil},
		{input: nil, inputErr: inpErr, err: inpErr, u: nil},
		{input: "", err: errors.New("Invalid type for user"), u: nil},
		{
			input: map[string]interface{}{"id": 1},
			err:   errors.New("Error fetching field"),
			u:     &User{Id: nil, Email: nil, Username: nil, IP: nil, UserAgent: nil},
		},
		{
			input: map[string]interface{}{
				"id": id, "email": mail, "username": name,
				"ip": ip, "user-agent": agent,
			},
			err: nil,
			u: &User{
				Id: &id, Email: &mail, Username: &name, IP: &ip, UserAgent: &agent,
			},
		},
	} {
		user, err := DecodeUser(test.input, test.inputErr)
		assert.Equal(t, test.u, user)
		assert.Equal(t, test.err, err)
	}
}

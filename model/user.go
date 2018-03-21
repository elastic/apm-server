package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type User struct {
	Id        *string
	Email     *string
	Username  *string
	IP        *string
	UserAgent *string
}

func DecodeUser(input interface{}, err error) (*User, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for user")
	}
	decoder := utility.ManualDecoder{}
	user := User{
		Id:        decoder.StringPtr(raw, "id"),
		Email:     decoder.StringPtr(raw, "email"),
		Username:  decoder.StringPtr(raw, "username"),
		IP:        decoder.StringPtr(raw, "ip"),
		UserAgent: decoder.StringPtr(raw, "user-agent"),
	}
	return &user, decoder.Err
}

func (u *User) Transform() common.MapStr {
	if u == nil {
		return nil
	}
	user := common.MapStr{}
	utility.Add(user, "id", u.Id)
	utility.Add(user, "email", u.Email)
	utility.Add(user, "username", u.Username)
	utility.Add(user, "ip", u.IP)
	utility.Add(user, "user-agent", u.UserAgent)
	return user
}

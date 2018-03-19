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
	df := utility.DataFetcher{}
	user := User{
		Id:        df.StringPtr(raw, "id"),
		Email:     df.StringPtr(raw, "email"),
		Username:  df.StringPtr(raw, "username"),
		IP:        df.StringPtr(raw, "ip"),
		UserAgent: df.StringPtr(raw, "user_agent"),
	}
	return &user, df.Err
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
	utility.Add(user, "user_agent", u.UserAgent)
	return user
}

package model

import (
	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type User struct {
	Id        *string `json:"id"`
	Email     *string `json:"email"`
	Username  *string `json:"username"`
	IP        *string
	UserAgent *string
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

func (u *User) Enrich(input pr.Intake) *User {
	if input.UserIP == "" && input.UserAgent == "" {
		return u
	}
	if u == nil {
		u = &User{}
	}
	if input.UserIP != "" {
		u.IP = &input.UserIP
	}
	if input.UserAgent != "" {
		u.UserAgent = &input.UserAgent
	}
	return u
}

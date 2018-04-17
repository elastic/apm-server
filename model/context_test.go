package model

var (
	ip  = "127.0.0.1"
	id  = "123"
	pid = 123
)

// func TestTransformContext(t *testing.T) {
// 	tests := []struct {
// 		process *Process
// 		system  *System
// 		service *Service
// 		user    *User
// 		context *TransformContext
// 	}{
// 		{
// 			process: nil,
// 			system:  nil,
// 			service: nil,
// 			user:    nil,
// 			context: &TransformContext{},
// 		},
// 		{
// 			process: &Process{},
// 			system:  &System{},
// 			service: &Service{},
// 			user:    &User{},
// 			context: &TransformContext{
// 				process: common.MapStr{"pid": 0},
// 				service: common.MapStr{"name": "", "agent": common.MapStr{"version": "", "name": ""}},
// 				system:  common.MapStr{},
// 				user:    common.MapStr{},
// 			},
// 		},
// 		{
// 			process: &Process{Pid: pid},
// 			system:  &System{IP: &ip},
// 			service: &Service{Name: "service"},
// 			user:    &User{Id: &id},
// 			context: &TransformContext{
// 				process: common.MapStr{"pid": 123},
// 				system:  common.MapStr{"ip": ip},
// 				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
// 				user:    common.MapStr{"id": "123"},
// 			},
// 		},
// 	}

// 	for idx, te := range tests {
// 		ctx := NewTransformContext(te.service, te.process, te.system, te.user)
// 		assert.Equal(t, te.context, ctx,
// 			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.context, ctx))
// 	}
// }

// func TestTransformContextTransform(t *testing.T) {

// 	tests := []struct {
// 		context *TransformContext
// 		m       common.MapStr
// 		out     common.MapStr
// 	}{
// 		{
// 			context: &TransformContext{},
// 			m:       common.MapStr{},
// 			out:     common.MapStr{},
// 		},
// 		{
// 			context: &TransformContext{},
// 			m:       common.MapStr{"user": common.MapStr{"id": 123}},
// 			out:     common.MapStr{"user": common.MapStr{"id": 123}},
// 		},
// 		{
// 			context: &TransformContext{
// 				process: common.MapStr{"pid": 123},
// 				system:  common.MapStr{"ip": ip},
// 				service: common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
// 				user:    common.MapStr{"id": 456},
// 			},
// 			m: common.MapStr{"foo": "bar", "user": common.MapStr{"id": 123, "username": "foo"}},
// 			out: common.MapStr{
// 				"foo":     "bar",
// 				"user":    common.MapStr{"id": 456, "username": "foo"},
// 				"process": common.MapStr{"pid": 123},
// 				"system":  common.MapStr{"ip": ip},
// 				"service": common.MapStr{"name": "service", "agent": common.MapStr{"version": "", "name": ""}},
// 			},
// 		},
// 	}

// 	for idx, te := range tests {
// 		out := te.context.Transform(te.m)
// 		assert.Equal(t, te.out, out,
// 			fmt.Sprintf("<%v> Expected: %v, Actual: %v", idx, te.out, out))
// 	}
// }

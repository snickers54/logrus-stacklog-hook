package stklog

const KEY_PROJECT_KEY = "project_key"

type Options map[string]interface{}

// empty interface, but I prefer defining it
type iEvents interface{}

// interface to abstract transport usage
type iTransport interface {
	Send()
	Init()
	Flush()
	GetOption(string) (interface{}, bool)
}

type transport struct {
	options    Options
	projectKey string
}

func (self *transport) GetProjectKey() string {
	if valueInterface, ok := self.GetOption(KEY_PROJECT_KEY); ok {
		switch valueInterface.(type) {
		case string:
			return valueInterface.(string)
		default:
			break
		}
	}
	return ""
}
func (self *transport) GetOption(keyName string) (value interface{}, ok bool) {
	value, ok = self.options[keyName]
	return
}

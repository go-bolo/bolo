package bolo

type Context interface {
	Get(key string) interface{}
	Set(key string, val interface{})
}

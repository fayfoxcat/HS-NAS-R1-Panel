//go:build !linux

package internal

func getStorage() []map[string]interface{} {
	return []map[string]interface{}{}
}

func getDiskHealth() []map[string]interface{} {
	return []map[string]interface{}{}
}

package env

import "flag"

func init() {

}

func IsTest() bool {
	return flag.Lookup("test.v") == nil
}

package props

func GetPropFunc(sproutID string) func(string) string {
	return func(name string) string {
		return GetProp(sproutID, name)
	}
}

// TODO: implement
func GetProp(sproutID, name string) string {
	return "props"
}

func SetPropFunc(sproutID string) func(string, string) error {
	return func(name, value string) error {
		return SetProp(sproutID, name, value)
	}
}

// TODO: implement
func SetProp(sproutID, name, value string) error {
	return nil
}

func GetDeletePropFunc(sproutID string) func(string) error {
	return func(name string) error {
		return DeleteProp(sproutID, name)
	}
}

// TODO: implement
func DeleteProp(sproutID, name string) error {
	return nil
}

func GetPropsFunc(sproutID string) func() map[string]string {
	return func() map[string]string {
		return GetProps(sproutID)
	}
}

// TODO: implement
func GetProps(sproutID string) map[string]string {
	return nil
}

func GetHostnameFunc(sproutID string) func() string {
	return func() string {
		return Hostname(sproutID)
	}
}

// TODO: implement
func Hostname(sproutID string) string {
	return "hostname"
}

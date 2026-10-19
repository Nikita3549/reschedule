package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

func ParseConfig(cfg any) []error {
	t := reflect.TypeOf(cfg).Elem()
	v := reflect.ValueOf(cfg).Elem()
	var errs []error

	for field := range t.Fields() {
		kind := field.Type.Kind()

		var envTag string
		var value string

		if kind != reflect.Ptr {
			envTag = field.Tag.Get("env")
			isOptional := field.Tag.Get("optional") == "true"
			if envTag == "" {
				errs = append(errs, fmt.Errorf("missing env tag for var %s", field.Name))
				continue
			}

			value = os.Getenv(envTag)
			if value == "" && !isOptional {
				errs = append(errs, fmt.Errorf("missing env variable %s", envTag))
				continue
			}
		}

		f := v.FieldByName(field.Name)
		switch kind {
		case reflect.Int:
			valueInt, err := strconv.Atoi(value)
			if err != nil {
				errs = append(errs, fmt.Errorf("env variable %s: invalid value %q, expected type int", envTag, value))
				continue
			}
			f.SetInt(int64(valueInt))

		case reflect.String:
			f.SetString(value)

		case reflect.Bool:
			valueBool, err := strconv.ParseBool(value)
			if err != nil {
				errs = append(errs, fmt.Errorf("env variable %s: invalid value %q, expected type bool", envTag, value))
				continue
			}
			f.SetBool(valueBool)

		case reflect.Ptr:
			f.Set(reflect.New(field.Type.Elem()))
			nestedErrs := ParseConfig(f.Interface())
			errs = append(errs, nestedErrs...)

		default:
			errs = append(errs, fmt.Errorf("field %s has unsupported type %s", envTag, kind))
		}
	}

	return errs
}

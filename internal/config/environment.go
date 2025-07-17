package config

import (
	"os"
	"strconv"
	"time"

	"github.com/drone/envsubst"
	"github.com/pkg/errors"

	"github.com/goccy/go-yaml"
)

type InterpolatedString string

// UnmarshalYAML implements yaml.InterfaceUnmarshaler.
func (is *InterpolatedString) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err != nil {
		return errors.WithStack(err)
	}

	str, err := envsubst.Eval(str, getEnv)
	if err != nil {
		return errors.WithStack(err)
	}

	*is = InterpolatedString(str)

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedString)

type InterpolatedInt int

func (ii *InterpolatedInt) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err != nil {
		return errors.WithStack(err)
	}

	str, err := envsubst.Eval(str, getEnv)
	if err != nil {
		return errors.WithStack(err)
	}

	intVal, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return errors.WithStack(err)
	}

	*ii = InterpolatedInt(int(intVal))

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedInt)

type InterpolatedFloat float64

func (ifl *InterpolatedFloat) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err != nil {
		return errors.WithStack(err)
	}

	str, err := envsubst.Eval(str, getEnv)
	if err != nil {
		return errors.WithStack(err)
	}

	floatVal, err := strconv.ParseFloat(str, 32)
	if err != nil {
		return errors.WithStack(err)
	}

	*ifl = InterpolatedFloat(floatVal)

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedFloat)

type InterpolatedBool bool

func (ib *InterpolatedBool) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err != nil {
		return errors.WithStack(err)
	}

	str, err := envsubst.Eval(str, getEnv)
	if err != nil {
		return errors.WithStack(err)
	}

	boolVal, err := strconv.ParseBool(str)
	if err != nil {
		return errors.WithStack(err)
	}

	*ib = InterpolatedBool(boolVal)

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedBool)

var getEnv = os.Getenv

type InterpolatedMap struct {
	Data map[string]any
}

func (im *InterpolatedMap) UnmarshalYAML(unmarshal func(any) error) error {
	var data map[string]any

	if err := unmarshal(&data); err != nil {
		return errors.WithStack(err)
	}

	interpolated, err := im.interpolateRecursive(data)
	if err != nil {
		return errors.WithStack(err)
	}

	im.Data = interpolated.(map[string]any)

	return nil
}

func (im *InterpolatedMap) MarshalYAML() (any, error) {
	return im.Data, nil
}

func (im InterpolatedMap) interpolateRecursive(data any) (any, error) {
	switch typ := data.(type) {
	case map[string]any:
		for key, value := range typ {
			value, err := im.interpolateRecursive(value)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			typ[key] = value
		}

	case string:
		value, err := envsubst.Eval(typ, getEnv)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		data = value

	case []any:
		for idx := range typ {
			value, err := im.interpolateRecursive(typ[idx])
			if err != nil {
				return nil, errors.WithStack(err)
			}

			typ[idx] = value
		}
	}

	return data, nil
}

type InterpolatedStringSlice []string

func (iss *InterpolatedStringSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var data []string

	if err := unmarshal(&data); err != nil {
		return errors.WithStack(err)
	}

	for index, value := range data {
		value, err := envsubst.Eval(value, getEnv)
		if err != nil {
			return errors.WithStack(err)
		}

		data[index] = value
	}

	*iss = data

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedStringSlice)

type InterpolatedDuration time.Duration

func (id *InterpolatedDuration) UnmarshalYAML(unmarshal func(any) error) error {
	var str string

	if err := unmarshal(&str); err != nil {
		return errors.WithStack(err)
	}

	str, err := envsubst.Eval(str, getEnv)
	if err != nil {
		return errors.WithStack(err)
	}

	duration, err := time.ParseDuration(str)
	if err != nil {
		nanoseconds, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return errors.WithStack(err)
		}

		duration = time.Duration(nanoseconds)
	}

	*id = InterpolatedDuration(duration)

	return nil
}

var _ yaml.InterfaceUnmarshaler = new(InterpolatedDuration)

func (id *InterpolatedDuration) MarshalYAML() (any, error) {
	duration := time.Duration(*id)

	return duration.String(), nil
}

var _ yaml.InterfaceMarshaler = new(InterpolatedDuration)

func NewInterpolatedDuration(d time.Duration) *InterpolatedDuration {
	id := InterpolatedDuration(d)
	return &id
}

package config

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

func TestInterpolatedMap(t *testing.T) {
	type testCase struct {
		Path   string
		Env    map[string]string
		Assert func(t *testing.T, parsed InterpolatedMap)
	}

	testCases := []testCase{
		{
			Path: "testdata/environment/interpolated-map-1.yml",
			Env: map[string]string{
				"TEST_PROP1":      "foo",
				"TEST_SUB_PROP1":  "bar",
				"TEST_SUB2_PROP1": "baz",
			},
			Assert: func(t *testing.T, parsed InterpolatedMap) {
				if e, g := "foo", parsed.Data["prop1"]; e != g {
					t.Errorf("parsed.Data[\"prop1\"]: expected '%v', got '%v'", e, g)
				}

				if e, g := "bar", parsed.Data["sub"].(map[string]any)["subProp1"]; e != g {
					t.Errorf("parsed.Data[\"sub\"][\"subProp1\"]: expected '%v', got '%v'", e, g)
				}

				if e, g := "baz", parsed.Data["sub2"].(map[string]any)["sub2Prop1"].([]any)[0]; e != g {
					t.Errorf("parsed.Data[\"sub2\"][\"sub2Prop1\"][0]: expected '%v', got '%v'", e, g)
				}

				if e, g := "test", parsed.Data["sub2"].(map[string]any)["sub2Prop1"].([]any)[1]; e != g {
					t.Errorf("parsed.Data[\"sub2\"][\"sub2Prop1\"][1]: expected '%v', got '%v'", e, g)
				}
			},
		},
		{
			Path: "testdata/environment/interpolated-map-2.yml",
			Env: map[string]string{
				"BAR": "bar",
			},
			Assert: func(t *testing.T, parsed InterpolatedMap) {
				if e, g := "http://bar", parsed.Data["foo"]; e != g {
					t.Errorf("parsed.Data[\"foo\"]: expected '%v', got '%v'", e, g)
				}
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			data, err := os.ReadFile(tc.Path)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			var interpolatedMap InterpolatedMap

			if tc.Env != nil {
				getEnv = func(key string) string {
					return tc.Env[key]
				}
			}

			if err := yaml.Unmarshal(data, &interpolatedMap); err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			if tc.Assert != nil {
				tc.Assert(t, interpolatedMap)
			}
		})
	}
}

func TestInterpolatedDuration(t *testing.T) {
	type testCase struct {
		Path   string
		Env    map[string]string
		Assert func(t *testing.T, parsed *InterpolatedDuration)
	}

	testCases := []testCase{
		{
			Path: "testdata/environment/interpolated-duration.yml",
			Env: map[string]string{
				"MY_DURATION": "30s",
			},
			Assert: func(t *testing.T, parsed *InterpolatedDuration) {
				if e, g := 30*time.Second, parsed; e != time.Duration(*g) {
					t.Errorf("parsed: expected '%v', got '%v'", e, g)
				}
			},
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			data, err := os.ReadFile(tc.Path)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			if tc.Env != nil {
				getEnv = func(key string) string {
					return tc.Env[key]
				}
			}

			config := struct {
				Duration *InterpolatedDuration `yaml:"duration"`
			}{
				Duration: NewInterpolatedDuration(-1),
			}

			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			if tc.Assert != nil {
				tc.Assert(t, config.Duration)
			}
		})
	}
}

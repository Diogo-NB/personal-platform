package config

import "testing"

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		environment map[string]string
		want        Config
		wantError   bool
	}{
		{
			name: "valid environment",
			environment: map[string]string{
				"CLUSTER_ARN":  "cluster",
				"SERVICE_NAME": "service",
			},
			want: Config{ClusterARN: "cluster", ServiceName: "service"},
		},
		{
			name: "missing cluster arn",
			environment: map[string]string{
				"SERVICE_NAME": "service",
			},
			wantError: true,
		},
		{
			name: "missing service name",
			environment: map[string]string{
				"CLUSTER_ARN": "cluster",
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := load(func(key string) string { return test.environment[key] })
			if (err != nil) != test.wantError {
				t.Fatalf("load() error = %v, wantError %t", err, test.wantError)
			}
			if got != test.want {
				t.Errorf("load() = %#v, want %#v", got, test.want)
			}
		})
	}
}

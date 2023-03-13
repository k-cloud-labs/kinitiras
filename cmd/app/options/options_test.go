package options

import (
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

func TestPrintFlags(t *testing.T) {
	type args struct {
		flags *pflag.FlagSet
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "1",
			args: args{flags: pflag.NewFlagSet("test1", pflag.ExitOnError)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := NewOptions()
			opt.AddFlags(tt.args.flags)
			PrintFlags(tt.args.flags)
			opt.PreCacheResourcesToGVKList()
		})
	}
}

func TestResourceSlice_Set(t *testing.T) {
	type fields struct {
		s *ResourceSlice
	}
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "1",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				val: "Node/v1",
			},
			wantErr: false,
		},
		{
			name: "error",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				val: "v1",
			},
			wantErr: true,
		},
		{
			name: "error2",
			fields: fields{
				s: &ResourceSlice{},
			},
			args: args{
				val: "v1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fields.s.Set(tt.args.val); (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
			t.Log(tt.fields.s.Type())
		})
	}
}

func TestResourceSlice_Append(t *testing.T) {
	type fields struct {
		s *ResourceSlice
	}
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "1",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				val: "Node/v1",
			},
			wantErr: false,
		},
		{
			name: "error",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				val: "Node",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.s
			if err := s.Append(tt.args.val); (err != nil) != tt.wantErr {
				t.Errorf("Append() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResourceSlice_GetSlice(t *testing.T) {
	type fields struct {
		s *ResourceSlice
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "1",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1", "Deployment/apps/v1"}),
			},
			want: []string{"Pod/v1", "Deployment/apps/v1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.s
			if got := s.GetSlice(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceSlice_Replace(t *testing.T) {
	type fields struct {
		s *ResourceSlice
	}
	type args struct {
		slice []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "1",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				slice: []string{"Node/v1"},
			},
			wantErr: false,
		},
		{
			name: "error",
			fields: fields{
				s: NewPreCacheResources([]string{"Pod/v1"}),
			},
			args: args{
				slice: []string{"Node"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fields.s
			if err := s.Replace(tt.args.slice); (err != nil) != tt.wantErr {
				t.Errorf("Replace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

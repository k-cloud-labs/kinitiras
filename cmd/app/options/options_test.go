package options

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestOptions_PreCacheResourcesToGVKList(t *testing.T) {
	type fields struct {
		PreCacheResources string
	}
	tests := []struct {
		name   string
		fields fields
		want   []schema.GroupVersionKind
	}{
		{
			name:   "empty",
			fields: fields{},
			want:   make([]schema.GroupVersionKind, 0),
		},
		{
			name: "pod",
			fields: fields{
				PreCacheResources: "Pod/v1",
			},
			want: []schema.GroupVersionKind{
				{
					Version: "v1",
					Kind:    "Pod",
				},
			},
		},
		{
			name: "Deployment",
			fields: fields{
				PreCacheResources: "Pod/apps/v1",
			},
			want: []schema.GroupVersionKind{
				{
					Group:   "apps",
					Version: "v1",
					Kind:    "Pod",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Options{
				PreCacheResources: tt.fields.PreCacheResources,
			}
			if got := o.PreCacheResourcesToGVKList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PreCacheResourcesToGVKList() = %v, want %v", got, tt.want)
			}
		})
	}
}

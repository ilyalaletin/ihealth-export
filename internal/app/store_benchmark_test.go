package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkApplyBatch(b *testing.B) {
	for _, size := range []int{500, 2000, 5000} {
		b.Run(fmt.Sprintf("samples_%d", size), func(b *testing.B) {
			store, err := OpenStore(filepath.Join(b.TempDir(), "bench.db"))
			if err != nil {
				b.Fatal(err)
			}
			defer store.Close()

			value := 1.0
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				samples := make([]Sample, size)
				for index := range samples {
					samples[index] = Sample{
						ID:        fmt.Sprintf("%d-%d", iteration, index),
						Type:      "HKQuantityTypeIdentifierActiveEnergyBurned",
						Kind:      "quantity",
						StartDate: "2026-08-27T00:00:00Z",
						EndDate:   "2026-08-27T00:01:00Z",
						Value:     &value,
						Unit:      "kcal",
					}
				}
				if _, err := store.ApplyBatch(context.Background(), UploadBatch{DeviceID: "phone", Type: "benchmark", Samples: samples}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

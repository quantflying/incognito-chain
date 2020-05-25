package multiview

import (
	"testing"
)

func TestLoadKeyList(t *testing.T) {
	LoadKeyList("keylist.json")
}

func TestGenPayload(t *testing.T) {
	type args struct {
		filePriKey string
		pubGroup   map[int]map[int][]int
		fileOut    string
		From       map[int]uint64
		To         map[int]uint64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "1",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						3: []int{
							5,
						},
						5: []int{
							3,
						},
					},
				},
				From: map[int]uint64{
					255: 2,
				},
				To: map[int]uint64{
					255: 2,
				},
			},
		},
		{
			name: "1",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						1: []int{
							2, 4, 6, 7,
						},
						2: []int{
							1, 4, 6, 7,
						},
						4: []int{
							1, 2, 6, 7,
						},
						6: []int{
							1, 2, 4, 7,
						},
						7: []int{
							1, 2, 4, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 3,
				},
				To: map[int]uint64{
					255: 3,
				},
			},
		},
		{
			name: "1",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						1: []int{
							2, 3, 4, 5, 7,
						},
						2: []int{
							1, 3, 4, 5, 7,
						},
						3: []int{
							1, 2, 4, 5, 7,
						},
						4: []int{
							1, 2, 6, 7,
						},
						5: []int{
							1, 2, 3, 4, 7,
						},
						7: []int{
							1, 2, 4, 5, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 4,
				},
				To: map[int]uint64{
					255: 4,
				},
			},
		},
		{
			name: "1",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{},
				},
				From: map[int]uint64{
					255: 6,
				},
				To: map[int]uint64{
					255: 6,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GenPayload(tt.args.filePriKey, tt.args.pubGroup, tt.args.From, tt.args.To, tt.args.fileOut)
		})
	}
}

func TestGenPayloadV2(t *testing.T) {
	type args struct {
		filePriKey string
		pubGroup   map[int]map[int][]int
		fileOut    string
		From       map[int]uint64
		To         map[int]uint64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "T2",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						2: []int{
							4,
						},
						4: []int{
							2,
						},
					},
				},
				From: map[int]uint64{
					255: 2,
				},
				To: map[int]uint64{
					255: 2,
				},
			},
		},
		{
			name: "T3",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						3: []int{
							1, 2, 4, 5, 6, 7,
						},
						1: []int{
							6,
						},
						2: []int{
							6,
						},
						4: []int{
							6,
						},
						5: []int{
							6,
						},
						7: []int{
							6,
						},
					},
				},
				From: map[int]uint64{
					255: 3,
				},
				To: map[int]uint64{
					255: 3,
				},
			},
		},
		{
			name: "T4",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						1: []int{
							2, 3, 4, 5, 7,
						},
						2: []int{
							1, 3, 4, 5, 7,
						},
						3: []int{
							1, 2, 4, 5, 7,
						},
						4: []int{
							1, 2, 3, 5, 7,
						},
						5: []int{
							1, 2, 3, 4, 7,
						},
						7: []int{
							1, 2, 3, 4, 5,
						},
					},
				},
				From: map[int]uint64{
					255: 4,
				},
				To: map[int]uint64{
					255: 4,
				},
			},
		},
		{
			name: "T5",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						5: []int{
							7,
						},
						7: []int{
							5,
						},
					},
				},
				From: map[int]uint64{
					255: 5,
				},
				To: map[int]uint64{
					255: 5,
				},
			},
		},
		{
			name: "T6",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						6: []int{
							1, 2, 4, 5, 6, 7,
						},
						1: []int{
							6,
						},
						2: []int{
							6,
						},
						4: []int{
							6,
						},
						5: []int{
							6,
						},
						7: []int{
							6,
						},
					},
				},
				From: map[int]uint64{
					255: 6,
				},
				To: map[int]uint64{
					255: 6,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GenPayload(tt.args.filePriKey, tt.args.pubGroup, tt.args.From, tt.args.To, tt.args.fileOut)
		})
	}
}

func TestGenPayloadV3(t *testing.T) {
	type args struct {
		filePriKey string
		pubGroup   map[int]map[int][]int
		fileOut    string
		From       map[int]uint64
		To         map[int]uint64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "T1",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						2: []int{
							4,
						},
						4: []int{
							2,
						},
					},
				},
				From: map[int]uint64{
					255: 1,
				},
				To: map[int]uint64{
					255: 1,
				},
			},
		},
		{
			name: "T2",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						3: []int{
							5, 6,
						},
						1: []int{
							5, 6,
						},
						5: []int{
							5, 6,
						},
						7: []int{
							5, 6,
						},
						6: []int{
							5,
						},
					},
				},
				From: map[int]uint64{
					255: 2,
				},
				To: map[int]uint64{
					255: 2,
				},
			},
		},
		{
			name: "T4",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						5: []int{
							7,
						},
						7: []int{
							5,
						},
					},
				},
				From: map[int]uint64{
					255: 4,
				},
				To: map[int]uint64{
					255: 4,
				},
			},
		},
		{
			name: "T5",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						6: []int{
							1, 2, 3, 4,
						},
						1: []int{
							2, 3, 4, 6,
						},
						2: []int{
							1, 6, 3, 4,
						},
						3: []int{
							1, 2, 6, 4,
						},
						4: []int{
							1, 2, 3, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 5,
				},
				To: map[int]uint64{
					255: 5,
				},
			},
		},
		{
			name: "T7",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						1: []int{
							3,
						},
						3: []int{
							1,
						},
					},
				},
				From: map[int]uint64{
					255: 7,
				},
				To: map[int]uint64{
					255: 7,
				},
			},
		},
		{
			name: "T8",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						2: []int{
							4, 5, 6, 7,
						},
						4: []int{
							2, 5, 6, 7,
						},
						5: []int{
							2, 4, 6, 7,
						},
						6: []int{
							2, 4, 5, 7,
						},
						7: []int{
							2, 4, 5, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 8,
				},
				To: map[int]uint64{
					255: 8,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GenPayload(tt.args.filePriKey, tt.args.pubGroup, tt.args.From, tt.args.To, tt.args.fileOut)
		})
	}
}

func TestGenPayloadV4(t *testing.T) {
	type args struct {
		filePriKey string
		pubGroup   map[int]map[int][]int
		fileOut    string
		From       map[int]uint64
		To         map[int]uint64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "T2",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						2: []int{
							6,
						},
						6: []int{
							2,
						},
					},
				},
				From: map[int]uint64{
					255: 2,
				},
				To: map[int]uint64{
					255: 2,
				},
			},
		},
		{
			name: "T3",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						3: []int{
							1, 2, 5, 6, 7,
						},
						1: []int{
							2, 3, 5, 6, 7,
						},
						2: []int{
							1, 3, 5, 6, 7,
						},
						5: []int{
							1, 2, 3, 6, 7,
						},
						6: []int{
							1, 2, 3, 5, 7,
						},
						7: []int{
							1, 2, 3, 5, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 3,
				},
				To: map[int]uint64{
					255: 3,
				},
			},
		},
		{
			name: "T5",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						5: []int{
							1, 2, 3, 6, 7,
						},
						1: []int{
							2, 3, 5, 6, 7,
						},
						2: []int{
							1, 3, 5, 6, 7,
						},
						3: []int{
							1, 2, 5, 6, 7,
						},
						6: []int{
							1, 2, 3, 5, 7,
						},
						7: []int{
							1, 2, 3, 5, 6,
						},
					},
				},
				From: map[int]uint64{
					255: 5,
				},
				To: map[int]uint64{
					255: 5,
				},
			},
		},
		{
			name: "T7",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						7: []int{
							2,
						},
						2: []int{
							7,
						},
					},
				},
				From: map[int]uint64{
					255: 7,
				},
				To: map[int]uint64{
					255: 7,
				},
			},
		},
		{
			name: "T8",
			args: args{
				filePriKey: "keylist.json",
				pubGroup: map[int]map[int][]int{
					255: map[int][]int{
						1: []int{
							3, 4, 5, 6,
						},
						3: []int{
							1, 4, 5, 6,
						},
						4: []int{
							1, 3, 5, 6,
						},
						5: []int{
							1, 3, 4, 6,
						},
						6: []int{
							1, 3, 4, 5,
						},
					},
				},
				From: map[int]uint64{
					255: 8,
				},
				To: map[int]uint64{
					255: 8,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GenPayload(tt.args.filePriKey, tt.args.pubGroup, tt.args.From, tt.args.To, tt.args.fileOut)
		})
	}
}

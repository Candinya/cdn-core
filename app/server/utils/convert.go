package utils

func Int64Array2uint(arr []int64) []uint {
	var ids []uint
	for _, id := range arr {
		ids = append(ids, uint(id))
	}
	return ids
}

func UintArray2int64(arr []uint) []int64 {
	var ids []int64
	for _, id := range arr {
		ids = append(ids, int64(id))
	}
	return ids
}

package psx

func AlignTextData(exe *File, size int64) {
	if exe.Size()%size == 0 {
		return
	}
	pad := make([]byte, size-(exe.Size()%size))
	exe.Section("text").Data = append(exe.Section("text").Data, pad...)
	exe.TextSize += uint32(len(pad))
}

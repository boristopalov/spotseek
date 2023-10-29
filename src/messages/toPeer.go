package messages


func (mb *MessageBuilder) QueueUpload(filename string) []byte { 
	mb.AddString(filename)
	return mb.Build(43)
}

func (mb *MessageBuilder) UserInfoRequest() []byte {
	return mb.Build(15)
}
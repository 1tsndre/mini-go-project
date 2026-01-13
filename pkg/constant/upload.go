package constant

const (
	UploadDirPerm = 0755

	MsgFileTooLarge          = "%w: file size exceeds maximum allowed size of %d bytes"
	MsgExtensionNotAllowed   = "%w: file extension %s is not allowed"
	MsgCreateUploadDirFailed = "failed to create upload directory: %w"
	MsgCreateFileFailed      = "failed to create file: %w"
	MsgWriteFileFailed       = "failed to write file: %w"
	MsgFinalizeFileFailed    = "failed to finalize file: %w"
)

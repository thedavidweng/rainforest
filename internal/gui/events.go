package gui

type Emitter func(name string, data any)

const (
	EventTransferStage    = "transfer-stage"
	EventTransferProgress = "transfer-progress"
	EventBackupStatus     = "backup-status"
	EventAuthChanged      = "auth-changed"
	EventFilesDropped     = "files-dropped"
	EventNotification     = "notification"
)

type TransferDTO struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Stage            string `json:"stage"`
	SourceSummary    string `json:"sourceSummary"`
	DestNodeID       string `json:"destNodeId,omitempty"`
	DestLocalDir     string `json:"destLocalDir,omitempty"`
	ConflictPolicy   string `json:"conflictPolicy,omitempty"`
	BytesDone        int64  `json:"bytesDone"`
	BytesTotal       int64  `json:"bytesTotal"`
	ItemsDone        int    `json:"itemsDone"`
	ItemsSkipped     int    `json:"itemsSkipped"`
	ItemsFailed      int    `json:"itemsFailed"`
	ItemsUnconfirmed int    `json:"itemsUnconfirmed"`
	ItemsTotal       int    `json:"itemsTotal"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
	FinishedAt       string `json:"finishedAt,omitempty"`
}

type TransferItemDTO struct {
	ID           string `json:"id"`
	TransferID   string `json:"transferId"`
	LocalPath    string `json:"localPath"`
	Name         string `json:"name"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
	MD5          string `json:"md5,omitempty"`
	RemoteNodeID string `json:"remoteNodeId,omitempty"`
	Status       string `json:"status"`
	BytesDone    int64  `json:"bytesDone"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	UpdatedAt    string `json:"updatedAt"`
}

type TransferProgressEvent struct {
	Transfer TransferDTO     `json:"transfer"`
	Item     TransferItemDTO `json:"item"`
}

type FilesDroppedEvent struct {
	Paths []string `json:"paths"`
	X     int      `json:"x,omitempty"`
	Y     int      `json:"y,omitempty"`
}

type NotificationEvent struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	Level      string `json:"level"`
	TransferID string `json:"transferId,omitempty"`
}

package rpc

// ChunkType defines the type of a chunk.
type ChunkType rune

const (
	ChunkTypeProgress ChunkType = 'p'
	ChunkTypeResult   ChunkType = 'r'
	ChunkTypeError    ChunkType = 'e'
)

// Chunk is a response chunk sent from the Testground daemon to the Testground
// client. For a given request, clients should expect between 0 to `n`
// `progress` chunks, and exactly 1 `result` or `error` chunk before EOF.
type Chunk struct {
	Type    ChunkType   `json:"t"` // progress or result or error
	Payload interface{} `json:"p,omitempty"`
	Error   *Error      `json:"e,omitempty"`
}

type Error struct {
	Msg string `json:"m"`
}

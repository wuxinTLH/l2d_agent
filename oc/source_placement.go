package oc

// SourcePlacement stores pixel coordinates of a compact part, not inferred anatomy.
type SourcePlacement struct {
	AssetID       string `json:"assetId"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	CanvasWidth   int    `json:"canvasWidth"`
	CanvasHeight  int    `json:"canvasHeight"`
	VerifiedScale bool   `json:"verifiedScale"`
}

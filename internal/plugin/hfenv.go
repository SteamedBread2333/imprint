package plugin

import (
	"net/http"
	"os"
	"time"

	"github.com/SteamedBread2333/imprint/internal/loopback"
)

const defaultHFMirror = "https://hf-mirror.com"

// huggingFaceHubEnv returns env vars so hub downloads work behind mirrors and
// without XET/CAS (which 401s when HF_ENDPOINT points at hf-mirror.com).
func huggingFaceHubEnv() []string {
	out := []string{
		"HF_HUB_DISABLE_XET=1",
		"HF_HUB_ENABLE_HF_TRANSFER=0",
	}
	if v := os.Getenv("HF_ENDPOINT"); v != "" {
		return out
	}
	if v := os.Getenv("IMPRINT_HF_ENDPOINT"); v != "" {
		out = append(out, "HF_ENDPOINT="+v)
		return out
	}
	if !huggingfaceCoReachable() {
		out = append(out, "HF_ENDPOINT="+defaultHFMirror)
	}
	return out
}

func huggingfaceCoReachable() bool {
	req, err := http.NewRequest(http.MethodHead, "https://huggingface.co", nil)
	if err != nil {
		return false
	}
	resp, err := loopback.Client(8 * time.Second).Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode > 0 && resp.StatusCode < 500
}

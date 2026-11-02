package cli

import (
	"os"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/testhome"
)

func TestMain(m *testing.M) { os.Exit(testhome.Run(m)) }

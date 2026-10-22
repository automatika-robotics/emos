package installer

import (
	"os/exec"
	"path/filepath"
	"regexp"
)

// standardNVCC is where NVIDIA's installers put the CUDA compiler.
const standardNVCC = "/usr/local/cuda/bin/nvcc"

// CUDAToolkit is a CUDA toolkit installed on this machine.
type CUDAToolkit struct {
	// Root is the toolkit's directory, which holds bin/nvcc.
	Root string
	// Version is its major.minor version.
	Version string
}

// DetectCUDA returns the machine's CUDA toolkit, or nil when it has none.
func DetectCUDA() *CUDAToolkit { return detectCUDA(standardNVCC) }

var nvccRelease = regexp.MustCompile(`release (\d+\.\d+)`)

// detectCUDA asks the compiler at standard, or else the one on the PATH, for
// its version.
func detectCUDA(standard string) *CUDAToolkit {
	nvcc, err := exec.LookPath(standard)
	if err != nil {
		if nvcc, err = exec.LookPath("nvcc"); err != nil {
			return nil
		}
	}
	out, err := exec.Command(nvcc, "--version").Output()
	release := nvccRelease.FindSubmatch(out)
	if err != nil || release == nil {
		return nil
	}
	// The versioned directory a link such as /usr/local/cuda stands for, so a
	// build keeps pointing at the toolkit it was made with.
	if resolved, err := filepath.EvalSymlinks(nvcc); err == nil {
		nvcc = resolved
	}
	return &CUDAToolkit{Root: filepath.Dir(filepath.Dir(nvcc)), Version: string(release[1])}
}

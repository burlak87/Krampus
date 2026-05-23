package video

import "os/exec"

func CompressH265(
	input string,
	output string,
) error {

	cmd := exec.Command(
		"ffmpeg",
		"-i", input,
		"-c:v", "libx265",
		"-preset", "medium",
		"-crf", "28",
		output,
	)

	return cmd.Run()
}

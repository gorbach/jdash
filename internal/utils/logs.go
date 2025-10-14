package utils

import (
	"strings"
)

const esc = 0x1b

// StripANSISecrets removes ANSI escape sequences from Jenkins log output while keeping
// regular text, and it strips concealed segments emitted by the Jenkins credentials
// masking plugin. It returns the cleaned string and whether a conceal sequence
// is still active (spanning across chunks).
func StripANSISecrets(input string, concealActive bool) (string, bool) {
	if input == "" && !concealActive {
		return input, false
	}

	var builder strings.Builder
	builder.Grow(len(input))

	for i := 0; i < len(input); {
		b := input[i]

		switch b {
		case '\r':
			// Ignore carriage returns; treat them as line rewrites.
			i++
			continue

		case esc:
			if i+1 >= len(input) {
				i++
				continue
			}

			switch input[i+1] {
			case '[':
				end := i + 2
				for end < len(input) && (input[end] < '@' || input[end] > '~') {
					end++
				}
				if end >= len(input) {
					// Incomplete sequence; drop the rest.
					return builder.String(), concealActive
				}

				seq := input[i : end+1]
				i = end + 1

				if len(seq) >= 3 && seq[len(seq)-1] == 'm' {
					params := seq[2 : len(seq)-1]
					if params == "" {
						// Same as 0m; reset.
						concealActive = false
						continue
					}
					parts := strings.Split(params, ";")
					for _, part := range parts {
						switch part {
						case "8":
							// Conceal on.
							concealActive = true
						case "0", "00", "28":
							// Reset or explicit reveal.
							concealActive = false
						}
					}
					continue
				}

				// Other CSI sequences (cursor movement, clear line, etc.) are ignored.
				continue

			case ']':
				// OSC sequence: skip until BEL or ESC \
				end := i + 2
				for end < len(input) {
					if input[end] == '\a' {
						end++
						break
					}
					if input[end] == esc && end+1 < len(input) && input[end+1] == '\\' {
						end += 2
						break
					}
					end++
				}
				i = end
				continue

			default:
				// Unsupported, skip the escape char.
				i++
				continue
			}

		default:
			if concealActive {
				i++
				continue
			}
			builder.WriteByte(b)
			i++
		}
	}

	return builder.String(), concealActive
}

# Windows Language Switcher

Sometimes language switching is not working as expected in Windows 11.
This app simply does the job.
(at least for me)

## Features

- Custom keyboard shortcuts for language switching:
  - LeftAlt + Shift
  - Ctrl + Shift
- Start minimized
- Launch on system startup


## Installation

1. Download the latest release from the releases page
2. Run the executable


## Building from Source

1. Clone the repository
2. Install dependencies:
```bash
go mod download
```
3. Build the application:
```bash
go build -ldflags "-H windowsgui" -o lang-switcher.exe
```

## Configuration

The application stores its configuration in `config.json` in the same directory as the executable. The configuration includes:

- `SwitchOnAlt`: Boolean value determining whether to use LeftAlt+Shift (true) or Ctrl+Shift (false)

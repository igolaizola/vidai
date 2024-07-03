# vidai 📹🤖

**vidai** generates videos using AI.

This is a CLI tool for [RunwayML Gen-2](https://runwayml.com/) that adds some extra features on top of it.

> 📢 Connect with us! Join our Telegram group for support and collaboration: [t.me/igohub](https://t.me/igohub)

## 🚀 Features

- Generate videos directly from the command line using a text or image prompt.
- Use RunwayML's extend feature to generate longer videos.
- Create or extend videos longer than 4 seconds by reusing the last frame of the video as the input for the next generation.
- Other handy tools to edit videos, like generating loops or resizing videos.

## 📦 Installation

You can use the Golang binary to install **vidai**:

```bash
go install github.com/igopr/vidai/cmd/vidai@latest
```

Or you can download the binary from the [releases](https://github.com/igopr/vidai/releases)

## 📋 Requirements

You need to have a [RunwayML](https://runwayml.com/) account and extract the token from the request authorization header using your browser's developer tools.

To create extended videos, you need to have [ffmpeg](https://ffmpeg.org/) installed.

## 🕹️ Usage

### Some examples

Generate a video from an image prompt:

```bash
vidai generate --token RUNWAYML_TOKEN --image car.jpg --output car.mp4
```

Generate a video from a text prompt:

```bash
vidai generate --token RUNWAYML_TOKEN --text "a car in the middle of the road" --output car.mp4
```

Generate a video from a image prompt and extend it twice (using RunwayML's extend feature):

```bash
vidai generate --token RUNWAYML_TOKEN --image car.jpg --output car.mp4 --extend 2
```

Extend a video by reusing the last frame twice:

```bash
vidai extend --input car.mp4 --output car-extended.mp4 --n 2
```

Convert a video to a loop:

```bash
vidai loop --input car.mp4 --output car-loop.mp4
```

### Help

Launch `vidai` with the `--help` flag to see all available commands and options:

```bash
vidai --help
```

You can use the `--help` flag with any command to view available options:

```bash
vidai generate --help
```

### How to launch commands

Launch commands using a configuration file:

```bash
vidai generate --config vidai.conf
```

```bash
# vidai.conf
token RUNWAYML_TOKEN
image car.jpg
output car.mp4
extend 2
```

Using environment variables (`VIDAI` prefix, uppercase and underscores):

```bash
export VIDAI_TOKEN=RUNWAYML_TOKEN
export VIDAI_IMAGE="car.jpg"
export VIDAI_OUTPUT="car.mp4"
export VIDAI_EXTEND=2
vidai generate
```

Using command line arguments:

```bash
vidai generate --token RUNWAYML_TOKEN --image car.jpg --video car.mp4 --extend 2
```

## ⚠️ Disclaimer

The automation of RunwayML accounts is a violation of their Terms of Service and will result in your account(s) being terminated.

Read about RunwayML Terms of Service and Community Guidelines.

vidai was written as a proof of concept and the code has been released for educational purposes only. The authors are released of any liabilities which your usage may entail.

## 💖 Support

If you have found my code helpful, please give the repository a star ⭐

Additionally, if you would like to support my late-night coding efforts and the coffee that keeps me going, I would greatly appreciate a donation.

You can invite me for a coffee at ko-fi (0% fees):

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/igolaizola)

Or at buymeacoffee:

[![buymeacoffee](https://user-images.githubusercontent.com/11333576/223217083-123c2c53-6ab8-4ea8-a2c8-c6cb5d08e8d2.png)](https://buymeacoffee.com/igolaizola)

Donate to my PayPal:

[paypal.me/igolaizola](https://www.paypal.me/igolaizola)

Sponsor me on GitHub:

[github.com/sponsors/igolaizola](https://github.com/sponsors/igolaizola)

Or donate to any of my crypto addresses:

- BTC `bc1qvuyrqwhml65adlu0j6l59mpfeez8ahdmm6t3ge`
- ETH `0x960a7a9cdba245c106F729170693C0BaE8b2fdeD`
- USDT (TRC20) `TD35PTZhsvWmR5gB12cVLtJwZtTv1nroDU`
- USDC (BEP20) / BUSD (BEP20) `0x960a7a9cdba245c106F729170693C0BaE8b2fdeD`
- Monero `41yc4R9d9iZMePe47VbfameDWASYrVcjoZJhJHFaK7DM3F2F41HmcygCrnLptS4hkiJARCwQcWbkW9k1z1xQtGSCAu3A7V4`

Thanks for your support!

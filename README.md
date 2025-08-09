# whiskr

A simple, private, self-hosted web chat interface to interact with AI models via the OpenRouter API. All chat history and settings are stored locally in your browser, ensuring your privacy.

![screenshot](.github/chat.png)

## Features

* **Private & Self-Hosted:** Your conversations never leave your machine.
* **Broad Model Support:** Use any model available on your OpenRouter account.
* **Real-time Streaming:** Get responses from the AI as they are generated.
* **Full Conversation Control:** Edit, delete, or clear messages at any time.
* **Persistent Settings:** Remembers your chosen model, temperature, and other settings.

## Getting Started

1. **Set your API Key:** Copy the `.example.env` file to `.env` and add your `OPENROUTER_TOKEN`.
```bash
cp .example.env .env
```
2. **Build and Run:**
```bash
go build -o chat
./chat
```
3. Open your browser to `http://localhost:3443`.

## Usage

* **Send Message:** Type in the input box and press `Ctrl+Enter` or click the send button.
* **Edit/Delete:** Hover over a message to reveal the edit and delete options.
* **Options:** Change the model, temperature, prompt, or message role using the controls at the bottom-left.
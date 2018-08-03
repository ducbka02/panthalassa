# Events

- Messages

    - `MESSAGE:PERSISTED`
        - `message_id`
        - `partner` (hex encoded ed25519 public key)
        - `message`
            - `content` raw message content (will be "" if DApp message)
            - `created_at` unix timestamp

    - `MESSAGE:DELIVERED`
        - `message_id`
        - `partner` (hex encoded ed25519 public key)
        - `message`
            - `content` raw message content (will be "" if DApp message)
            - `created_at` unix timestamp


    - `MESSAGE:RECEIVED`
        - `message_id`
        - `partner` (hex encoded ed25519 public key)
        - `message`
            - `content` raw message content (will be "" if DApp message)
            - `created_at` unix timestamp

- DApp

    - `DAPP:PERSISTED`
        - `dapp_signing_key` hex encoded signing key used to sign the DApp
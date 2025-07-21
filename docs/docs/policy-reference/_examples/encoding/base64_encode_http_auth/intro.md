This example shows how `base64.encode` acts as a utility function to bridge communication between client and server when they don't speak the same language.

Some legacy client sends credentials in custom headers (`x-username`, `x-password`), but the downstream service expects HTTP Basic Authentication. This example policy uses the base64 function to deliver this transparently to the downstream caller.

**Why this is useful:** This pattern is common in API gateways where you need to adapt between different authentication schemes. The `base64.encode` function transforms the data into the format expected by the downstream service, enabling seamless integration.


This example shows how `base64.encode` acts as a utility function to bridge communication between client and server when they don't speak the same language.

Suppose that some legacy client sends credentials in custom headers (`x-username`, `x-password`), but the downstream service expects HTTP Basic Authentication. This example policy uses the base64 function to deliver this transparently to the downstream caller.

This might be useful in an API gateway where you need to adapt between different authentication schemes without the option of editing clients and downstream servers.

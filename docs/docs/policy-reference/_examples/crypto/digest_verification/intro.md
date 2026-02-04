This example shows how to use `crypto.md5` to verify payload integrity by computing a digest of the JSON representation and comparing it with an expected value.

Content verification is helpful where you need to ensure data hasn't been tampered with or missed during transmission. The digest acts as a fingerprint - any change to the payload will result in a different digest.

Change any value in the `payload` object (like the user name or resource path) and re-run the example. You'll see `digest_valid` becomes `false`, demonstrating how any change is detected.

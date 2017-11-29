package example

greeting = msg {
    concat("", ["Hello ", data.example.hostOS, "!"], msg)
}


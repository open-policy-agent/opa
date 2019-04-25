package example

greeting = msg {
    msg := concat("", ["Hello ", data.example.hostOS, "!"])
}


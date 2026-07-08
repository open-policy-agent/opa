package play

banned_words := {"hate", "kill"}

reasons contains word if {
	some word in banned_words
	contains(input.message, word)
}

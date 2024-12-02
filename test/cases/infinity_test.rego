package infinity_test

test_infinity {
    # Test Infinity string
    inf1 := to_number("Infinity")
    inf1 > 1e308  # Larger than max float64
    
    # Test Infinite string
    inf2 := to_number("Infinite")
    inf2 > 1e308  # Larger than max float64
    
    # Test that regular numbers still work
    to_number("123") == 123
    to_number("123.456") == 123.456
    
    # Test that invalid strings still return errors
    not to_number("not a number")
    not to_number("InfinityX")
    not to_number("Infinit")  # Missing 'e' or 'y'
}

test_infinity_operations {
    # Test arithmetic operations with infinity
    inf := to_number("Infinity")
    
    # Infinity should be greater than any finite number
    inf > 1e308
    inf > 1e100
    
    # Infinity should equal itself
    inf == inf
    
    # Test that infinity from both strings produces equal values
    to_number("Infinite") == to_number("Infinity")
}

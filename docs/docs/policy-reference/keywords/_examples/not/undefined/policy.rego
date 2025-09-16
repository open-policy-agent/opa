package play

deny contains "missing email" if not input.email

deny contains "under 18" if input.age < 18

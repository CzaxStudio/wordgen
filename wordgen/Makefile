BINARY=wordgen

build:
	go build -o $(BINARY) .

# Quick test runs
test-keyword:
	./$(BINARY) -mode=keyword -keywords=admin,john,root -leet -cap -years -special -output=test_keyword.txt

test-brute:
	./$(BINARY) -mode=brute -charset=digits -min=4 -max=4 -output=test_pins.txt

test-pattern:
	./$(BINARY) -mode=pattern -pattern=admin## -output=test_pattern.txt

test-combo:
	./$(BINARY) -mode=combinator -keywords=john,admin,root -output=test_combo.txt

clean:
	rm -f $(BINARY) *.txt

.PHONY: build test-keyword test-brute test-pattern test-combo clean

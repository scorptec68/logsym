QA test suite

* Creating a new test
- Run ./new to create a new test, NNN
- Put the expected output into a file called NNN.out
For example test 042 and output 042.out
- When creating test it will ask you for a group (like a tag),
which can be used when selectively running certain related tests.

* To run the tests 
- Run all tests
./check

- Run specific test
./check NNN  e.g. ./check 042

- Run a group of related tests
./check -g libcommon

* If the tests fail, then .bad files will be generated e.g. 042.out.bad
which can be used for comparison to the expected output

* group file contains the list of groups and what tests belong to it

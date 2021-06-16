/*
Package hashring implements consistent hashing hashring data structure.

In general, consistent hashing is all about mapping of object from a very big
set of values (e.g. request id) to object from a quite small set (e.g. server
address). The word "consistent" means that it can produce consistent mapping on
different machines or processes without additional state exchange and
communication.

For more theory about the subject please see this great document:
https://theory.stanford.edu/~tim/s16/l/l1.pdf

There are two goals for this hashring implementation:
1) To be efficient in highly concurrent applications by blocking read
operations for the least possible time.
2) To correctly handle very rare but yet possible hash collisions, which may
break all your eventually consistent application.

To reach the first goal hashring uses immutable AVL tree internally, making
read operations (getting item for object) blocked only for a tiny amount of
time needed to swap the ring's tree root after some write operation (insertion
or deletion).

The second goal is reached just by careful implementation and good tests
coverage of possible collision cases.
*/
package hashring

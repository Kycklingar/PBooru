package migrate

type fileQueue []fileIdentifier

func (q fileQueue) has(fid fileIdentifier) bool {
	for _, qfid := range q {
		if qfid == fid {
			return true
		}
	}

	return false
}

func (q *fileQueue) enqueue(fid fileIdentifier) {
	*q = append(*q, fid)
}

func (q fileQueue) next() bool {
	return len(q) > 0
}

func (q *fileQueue) dequeue() fileIdentifier {
	fid := (*q)[0]

	*q = (*q)[1:]

	return fid
}

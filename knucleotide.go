/*
The Computer Language Benchmarks Game - k-nucleotide
https://benchmarksgame.alioth.debian.org/u64q/knucleotide-description.html#knucleotide

Contributed by Olivier Mengué.
Based on work by Bert Gijsbers, Dirk Moerenhout, Jason Alan Palmer and the Go Authors.

Repo: https://github.com/dolmen-go/k-nucleotide
*/
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sort"
)

// seqString is a sequence of nucleotides as a string: "ACGT..."
type seqString string

// seqChars is a sequence of nucleotides as chars: 'A', 'C', 'G', 'T'...
type seqChars []byte

// seqBits is a sequence of nucleotides as 2 low bits per byte: 0, 1, 3, 2...
type seqBits []byte

// toBits converts *in-place*
func (seq seqChars) toBits() seqBits {
	for i := 0; i < len(seq); i++ {
		// 'A' => 0, 'C' => 1, 'T' => 2, 'G' => 3
		seq[i] = seq[i] >> 1 & 3
	}
	return seqBits(seq)
}

func (seq seqString) seqBits() seqBits {
	return seqChars(seq).toBits()
}

// seq32 is a short (<= 16) sequence of nucleotides in a compact form
// length is not embedded
type seq32 uint32

// seq64 is a short (17..32) sequence of nucleotides in a compact form
// length is not embedded
type seq64 uint64

// seq32 converts a seqBits to a seq32
func (seq seqBits) seq32() seq32 {
	var num seq32
	for _, char := range seq {
		num = num<<2 | seq32(char)
	}
	return num
}

// seq64 converts a seqBits to a seq64
func (seq seqBits) seq64() seq64 {
	var num seq64
	for _, char := range seq {
		num = num<<2 | seq64(char)
	}
	return num
}

// seqString converts a seq32 to a human readable string
func (num seq32) seqString(length int) seqString {
	sequence := make(seqChars, length)
	for i := 0; i < length; i++ {
		sequence[length-i-1] = "ACTG"[num&3]
		num = num >> 2
	}
	return seqString(sequence)
}

type counter uint32

type seqCount struct {
	seq   seqString
	count counter
}

// seqCounter is the common interface of seqCounts32 and seqCounts64
type seqCounter interface {
	countOf(seqString) counter
	sortedCounts(length int) []seqCount
}

func (dna seqBits) countSequences(length int) seqCounter {
	if length <= 16 {
		return dna._count32(length)
	} else {
		return dna._count64(length)
	}
}

type seqCounts32 map[seq32]*counter

var _ seqCounter = seqCounts32{}

func (dna seqBits) _count32(length int) seqCounts32 {
	counts := make(seqCounts32)
	key := dna[0 : length-1].seq32()
	mask := seq32(1)<<uint(2*length) - 1
	for index := length - 1; index < len(dna); index++ {
		key = key<<2&mask | seq32(dna[index])
		pointer := counts[key]
		if pointer == nil {
			n := counter(1)
			counts[key] = &n
		} else {
			*pointer++
		}
	}
	return counts
}

type seqCounts64 map[seq64]*counter

func (dna seqBits) _count64(length int) seqCounts64 {
	counts := make(seqCounts64)
	key := dna[0 : length-1].seq64()
	mask := seq64(1)<<uint(2*length) - 1
	for index := length - 1; index < len(dna); index++ {
		key = key<<2&mask | seq64(dna[index])
		pointer := counts[key]
		if pointer == nil {
			n := counter(1)
			counts[key] = &n
		} else {
			*pointer++
		}
	}
	return counts
}

func (counts seqCounts32) countOf(seq seqString) counter {
	p := counts[seq.seqBits().seq32()]
	if p == nil {
		return 0
	}
	return *p
}

func (counts seqCounts64) countOf(seq seqString) counter {
	p := counts[seq.seqBits().seq64()]
	if p == nil {
		return 0
	}
	return *p
}

func (counts seqCounts32) allCounts(length int) []seqCount {
	list := make([]seqCount, 0, len(counts))
	for key, counter := range counts {
		list = append(list, seqCount{key.seqString(length), *counter})
	}
	return list
}

// seqCountsDesc implements sort.Interface
type seqCountsDesc []seqCount

func (sc seqCountsDesc) Len() int { return len(sc) }

func (sc seqCountsDesc) Swap(i, j int) { sc[i], sc[j] = sc[j], sc[i] }

// Less order descending by count then seq
func (sc seqCountsDesc) Less(i, j int) bool {
	if sc[i].count == sc[j].count {
		return sc[i].seq > sc[j].seq
	}
	return sc[i].count > sc[j].count
}

func (counts seqCounts32) sortedCounts(length int) []seqCount {
	seqCounts := counts.allCounts(length)
	sort.Sort(seqCountsDesc(seqCounts))
	return seqCounts
}

func (counts seqCounts64) sortedCounts(length int) []seqCount {
	panic("not implemented")
}

type job struct {
	run    func(dna seqBits)
	result chan string
}

func makeJob(j func(dna seqBits) string) job {
	r := make(chan string, 1)
	return job{
		run: func(dna seqBits) {
			r <- j(dna)
		},
		result: r,
	}
}

func frequencyReportJob(length int) job {
	return makeJob(func(dna seqBits) string {
		return frequencyReport(dna, length)
	})
}

func sequenceReportJob(sequence seqString) job {
	return makeJob(func(dna seqBits) string {
		return sequenceReport(dna, sequence)
	})
}

var jobs = [...]job{
	frequencyReportJob(1),
	frequencyReportJob(2),
	sequenceReportJob("GGT"),
	sequenceReportJob("GGTA"),
	sequenceReportJob("GGTATT"),
	sequenceReportJob("GGTATTTTAATT"),
	sequenceReportJob("GGTATTTTAATTTATAGT"),
}

func main() {
	dna := readSequence(">THREE").toBits()

	queue := make(chan func(), len(jobs))
	for i := runtime.NumCPU(); i > 0; i-- {
		go func(q <-chan func()) {
			for j := range q {
				j()
			}
		}(queue)
	}

	// Queue all jobs
	for i := range jobs {
		// longest job first, shortest job last
		n := len(jobs) - 1 - i
		queue <- func() { jobs[n].run(dna) }
	}

	// Wait for results
	for i := range jobs {
		fmt.Println(<-jobs[i].result)
	}

	close(queue)
}

func readSequence(prefix string) (data seqChars) {
	// Find the sequence
	pfx := []byte(prefix)
	var lineCount int
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := in.ReadSlice('\n')
		if err != nil {
			panic("read error")
		}
		lineCount++
		if line[0] == '>' && bytes.HasPrefix(line, pfx) {
			break
		}
	}
	// Read the sequence
	data = make(seqChars, 0, lineCount*61)
	for {
		line, err := in.ReadSlice('\n')
		if len(line) <= 1 || line[0] == '>' {
			break
		}

		last := len(line) - 1
		if line[last] == '\n' {
			line = line[0:last]
		}
		data = append(data, seqChars(line)...)

		if err != nil {
			break
		}
	}
	return
}

func frequencyReport(dna seqBits, length int) string {
	counts := dna.countSequences(length)
	sequences := counts.sortedCounts(length)

	var buf bytes.Buffer
	buf.Grow((8 + length) * len(sequences))
	var scale float32 = 100.0 / float32(len(dna)-length+1)
	for _, sequence := range sequences {
		buf.WriteString(fmt.Sprintf(
			"%v %.3f\n", sequence.seq,
			float32(sequence.count)*scale),
		)
	}
	return buf.String()
}

func sequenceReport(dna seqBits, sequence seqString) string {
	counts := dna.countSequences(len(sequence))
	return fmt.Sprintf("%v\t%v", counts.countOf(sequence), sequence)
}

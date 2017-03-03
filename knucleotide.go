/* The Computer Language Benchmarks Game http://benchmarksgame.alioth.debian.org/
 *
 * contributed by Olivier Mengué.
 * based on work by Bert Gijsbers, Dirk Moerenhout, Jason Alan Palmer and the Go Authors.
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

// seqString converts a seq32 to a huùan readable string
func (num seq32) seqString(length int) seqString {
	sequence := make(seqChars, length)
	for i := 0; i < length; i++ {
		sequence[length-i-1] = "ACTG"[num&3]
		num = num >> 2
	}
	return seqString(sequence)
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
	dna := input()
	scheduler(dna)
	for i := range jobs {
		fmt.Println(<-jobs[i].result)
	}
}

func scheduler(dna seqBits) {
	command := make(chan int, len(jobs))
	for i := runtime.NumCPU(); i > 0; i-- {
		go worker(dna, command)
	}

	for i := range jobs {
		// longest job first, shortest job last
		command <- len(jobs) - 1 - i
	}
	close(command)
}

func worker(dna seqBits, command <-chan int) {
	for k := range command {
		jobs[k].run(dna)
	}
}

func input() (data seqBits) {
	return readSequence(">THREE").toBits()
}

func readSequence(prefix string) (data seqChars) {
	in, lineCount := findSequence(prefix)
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

func findSequence(prefix string) (in *bufio.Reader, lineCount int) {
	pfx := []byte(prefix)
	in = bufio.NewReaderSize(os.Stdin, 1<<20)
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
	return
}

type counter uint32

type sequence struct {
	nucs  seqString
	count counter
}

type sequenceSlice []sequence

func (ss sequenceSlice) Len() int {
	return len(ss)
}

func (ss sequenceSlice) Swap(i, j int) {
	ss[i], ss[j] = ss[j], ss[i]
}

func (ss sequenceSlice) Less(i, j int) bool {
	if ss[i].count == ss[j].count {
		return ss[i].nucs > ss[j].nucs
	}
	return ss[i].count > ss[j].count
}

func frequencyReport(dna seqBits, length int) string {
	var sortedSeqs sequenceSlice
	counts := count32(dna, length)
	for num, pointer := range counts {
		sortedSeqs = append(
			sortedSeqs,
			sequence{num.seqString(length), *pointer},
		)
	}
	sort.Sort(sortedSeqs)

	var buf bytes.Buffer
	buf.Grow((8 + length) * len(sortedSeqs))
	var scale float32 = 100.0 / float32(len(dna)-length+1)
	for _, sequence := range sortedSeqs {
		buf.WriteString(fmt.Sprintf(
			"%v %.3f\n", sequence.nucs,
			float32(sequence.count)*scale),
		)
	}
	return buf.String()
}

func sequenceReport(dna seqBits, sequence seqString) string {
	var pointer *counter
	seq := sequence.seqBits()
	if len(sequence) <= 16 {
		counts := count32(dna, len(sequence))
		pointer = counts[seq.seq32()]
	} else {
		counts := count64(dna, len(sequence))
		pointer = counts[seq.seq64()]
	}
	var sequenceCount counter
	if pointer != nil {
		sequenceCount = *pointer
	}
	return fmt.Sprintf("%v\t%v", sequenceCount, sequence)
}

func count32(dna seqBits, length int) map[seq32]*counter {
	counts := make(map[seq32]*counter)
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

func count64(dna seqBits, length int) map[seq64]*counter {
	counts := make(map[seq64]*counter)
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

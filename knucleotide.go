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

type job struct {
	run    func(dna []byte)
	result chan string
}

func makeJob(j func(dna []byte) string) job {
	r := make(chan string, 1)
	return job{
		run: func(dna []byte) {
			r <- j(dna)
		},
		result: r,
	}
}

func frequencyReportJob(length int) job {
	return makeJob(func(dna []byte) string {
		return frequencyReport(dna, length)
	})
}

func sequenceReportJob(sequence string) job {
	return makeJob(func(dna []byte) string {
		return sequenceReport(dna, sequence)
	})
}

var jobs = []job{
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

func scheduler(dna []byte) {
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

func worker(dna []byte, command <-chan int) {
	for k := range command {
		jobs[k].run(dna)
	}
}

func dnaToBits(data []byte) {
	for i := 0; i < len(data); i++ {
		// 'A' => 0, 'C' => 1, 'T' => 2, 'G' => 3
		data[i] = data[i] >> 1 & 3
	}
}

func input() (data []byte) {
	data = readSequence(">THREE")
	dnaToBits(data)
	return
}

func readSequence(prefix string) (data []byte) {
	in, lineCount := findSequence(prefix)
	data = make([]byte, 0, lineCount*61)
	for {
		line, err := in.ReadSlice('\n')
		if err != nil && (line == nil || len(line) == 0) || line[0] == '>' {
			break
		}
		last := len(line) - 1
		if line[last] == '\n' {
			line = line[0:last]
		}
		data = append(data, line...)
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
	nucs  string
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

func frequencyReport(dna []byte, length int) string {
	var sortedSeqs sequenceSlice
	counts := count32(dna, length)
	for num, pointer := range counts {
		sortedSeqs = append(
			sortedSeqs,
			sequence{decompress32(num, length), *pointer},
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

func sequenceReport(dna []byte, sequence string) string {
	var pointer *counter
	seq := []byte(sequence)
	dnaToBits(seq)
	if len(sequence) <= 16 {
		counts := count32(dna, len(sequence))
		pointer = counts[compress32(seq)]
	} else {
		counts := count64(dna, len(sequence))
		pointer = counts[compress64(seq)]
	}
	var sequenceCount counter
	if pointer != nil {
		sequenceCount = *pointer
	}
	return fmt.Sprintf("%v\t%v", sequenceCount, sequence)
}

func count32(dna []byte, length int) map[uint32]*counter {
	counts := make(map[uint32]*counter)
	key := compress32(dna[0 : length-1])
	mask := uint32(1)<<uint(2*length) - 1
	for index := length - 1; index < len(dna); index++ {
		key = key<<2&mask | uint32(dna[index])
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

func count64(dna []byte, length int) map[uint64]*counter {
	counts := make(map[uint64]*counter)
	key := compress64(dna[0 : length-1])
	mask := uint64(1)<<uint(2*length) - 1
	for index := length - 1; index < len(dna); index++ {
		key = key<<2&mask | uint64(dna[index])
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

func compress64(sequence []byte) uint64 {
	var num uint64
	for _, char := range sequence {
		num = num<<2 | uint64(char)
	}
	return num
}

func compress32(sequence []byte) uint32 {
	var num uint32
	for _, char := range sequence {
		num = num<<2 | uint32(char)
	}
	return num
}

func decompress32(num uint32, length int) string {
	sequence := make([]byte, length)
	for i := 0; i < length; i++ {
		sequence[length-i-1] = "ACTG"[num&3]
		num = num >> 2
	}
	return string(sequence)
}

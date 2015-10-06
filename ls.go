package main

import (
	"os"
	"github.com/timob/list"
	"fmt"
	"log"
	"strings"
	"syscall"
	"unsafe"
	"path"
	"github.com/bradfitz/slice"
)

type DisplayEntry struct {
	path string
	os.FileInfo
}

type DisplayEntryList struct {
	Data []DisplayEntry
	list.Slice
}

func getTermSize() (int, int, error) {
	var dimensions [4]uint16

	fd := os.Stdout.Fd()
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dimensions)), 0, 0, 0); err != 0 {
		return -1, -1, err
	}
	return int(dimensions[1]), int(dimensions[0]), nil
}

func strcmpi(a, b string) int {
	for i, av := range a {
		if i > len(b) {
			return 1
		}
		if av > 96 && av < 123 {
			av -= 32
		}
		bv := rune(b[i])
		if bv > 96 && bv < 123 {
			bv -= 32
		}

		if av != bv {
			if av > bv {
				return 1
			} else {
				return -1
			}
		}
	}

	if len(b) > len(a) {
		return -1
	} else {
		return 0
	}
}

func main() {
	files := list.NewSliceList(&list.StringSlice{Data:os.Args}).(*list.StringSlice)
	options := list.NewSliceList(&list.StringSlice{}).(*list.StringSlice)

	files.Remove(0)
	for iter := files.Iterator(0); iter.Next(); {
		if v := files.Data[iter.Pos()]; strings.HasPrefix(v, "-") {
			options.Data[options.Append()] = v
			iter.Remove()
			if v == "--" {
				break
			}
		}
	}

	if files.Len() == 0 {
		files.Data[files.Append()] = "."
	}

	var showDirEntries bool
	var showAll bool
	var showAlmostAll bool
	const (
		name int = iota
		modTime int = iota
		size int = iota
	)
	var sortType int = name
	var reverseSort bool
	for iter := options.Iterator(0); iter.Next(); {
		if option := options.Data[iter.Pos()]; !strings.HasPrefix(option, "--") && len(option) > 2 {
			letters := list.NewSliceList(&list.ByteSlice{Data:[]byte(option[1:])}).(*list.ByteSlice)
			var removed bool
			for iter2 := letters.Iterator(letters.Len() - 1); iter2.Prev(); {
				options.Data[iter.Insert()] = "-" + string(letters.Data[iter2.Pos()])
				if !removed {
					iter.Remove()
					removed = true
				}
				iter.Prev()
			}
		}

		switch options.Data[iter.Pos()] {
		case "-d":
			showDirEntries = true
		case "-a":
			showAll = true
		case "-A":
			showAlmostAll = true
			showAll = true
		case "-t":
			sortType = modTime
		case "-S":
			sortType = size
		case "-r":
			reverseSort = true
		default:
			log.Fatalf("unkown option %s", options.Data[iter.Pos()])
		}
	}

	var width int
	if w, _, err := getTermSize(); err == nil {
		width = w
	} else {
		width = 80
	}

	selected := list.NewSliceList(&DisplayEntryList{}).(*DisplayEntryList)
	for iter := files.Iterator(0); iter.Next(); {
		if fileName := files.Data[iter.Pos()]; showDirEntries {
			if stat, err := os.Lstat(fileName); err == nil {
				selected.Data[selected.Append()] = DisplayEntry{fileName, stat}
			} else {
				log.Print(err)
			}
		} else {
			if stat, err := os.Stat(fileName); err == nil {
				if stat.IsDir() {
					if file, err := os.Open(fileName); err == nil {
						if fileInfos, err := file.Readdir(0); err == nil {
							if showAll && !showAlmostAll {
								selected.Data[selected.Append()] = DisplayEntry{".", stat}
								if parent, err := os.Stat(path.Dir(fileName)); err == nil {
									selected.Data[selected.Append()] = DisplayEntry{"..", parent}
								} else {
									log.Print(err)
								}
							}
							for _, v := range fileInfos {
								if !strings.HasPrefix(v.Name(), ".") || showAll {
									selected.Data[selected.Append()] = DisplayEntry{v.Name(), v}
								}
							}
						} else {
							log.Print(err)
						}
					} else {
						log.Print(err)
					}
				} else {
					selected.Data[selected.Append()] = DisplayEntry{fileName, stat}
				}
			} else {
				log.Print(err)
			}
		}

		slice.Sort(selected.Data, func(i, j int) (v bool) {
			var same bool
			if sortType == modTime {
				v = selected.Data[i].ModTime().Before(selected.Data[j].ModTime())
				if !v {
					same = selected.Data[i].ModTime().Equal(selected.Data[j].ModTime())
				}
				v = !v
			} else if sortType == size {
				d := selected.Data[j].Size() - selected.Data[i].Size()
				if d > 0 {
					v = true
				} else if d == 0 {
					same = true
				}
				v = !v
			} else {
				// strcoll?
				v = strcmpi(selected.Data[i].path, selected.Data[j].path) == -1
			}
			if same {
				v = strcmpi(selected.Data[i].path, selected.Data[j].path) == -1
			} else if reverseSort {
				v = !v
			}
			return
		})

		padding := 2
		smallestWord := 1
		cols := width / (padding + smallestWord)
		colWidths := make([]int, cols)
A:
		for cols > 1 {
			colWidths = colWidths[:cols]
			for i := range colWidths {
				colWidths[i] = 0
			}
			pos := (cols - 1) * padding
			for i, v := range selected.Data {
				p := i % cols
				if len(v.path) > colWidths[p] {
					pos += len(v.path) - colWidths[p]
					if pos >= width {
						cols--
						continue A
					}
					colWidths[p] = len(v.path)
				}
			}
			break
		}

		for i, v := range selected.Data {
			w := colWidths[i % cols]
			if i % cols == 0 {
				if i != 0 {
					fmt.Println()
				}
			}
			fmt.Printf("%s", v.path)
			fmt.Print(strings.Repeat(" ", (w - len(v.path)) + padding))
		}

		fmt.Println()
		selected.Clear()
	}
}
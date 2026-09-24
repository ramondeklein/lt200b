package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"sync"
)

func main() {
	address := flag.String("address", "", "MAC address of printer")
	imagePath := flag.String("image", "", "Image file to print")
	text := flag.String("text", "", "Text to print")
	fontSize := flag.Int("font-size", 64, "Font size for text")
	listen := flag.String("listen", "", "Serve HTTP on this address (for example :8080)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Print image or text on a DYMO LetraTag 200B.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s --address aa:bb:cc:dd:ee:ff --text \"Hello World\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --address aa:bb:cc:dd:ee:ff --image hello.png\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --address aa:bb:cc:dd:ee:ff --listen :8080\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(*address, *imagePath, *text, *fontSize, *listen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var printMu sync.Mutex

func printImage(address string, img image.Image) error {
	printMu.Lock()
	defer printMu.Unlock()
	chunks, err := createJob(img)
	if err != nil {
		return err
	}
	return printJob(address, chunks)
}

func configFromEnv(address, listen string) (string, string) {
	if address == "" {
		address = os.Getenv("LT200B_ADDRESS")
	}
	if listen == "" {
		listen = os.Getenv("LT200B_LISTEN")
	}
	return address, listen
}

func run(address, imagePath, text string, fontSize int, listen string) error {
	address, listen = configFromEnv(address, listen)
	if address == "" {
		return fmt.Errorf("MAC address is required (--address)")
	}
	if listen != "" {
		if text != "" || imagePath != "" {
			return fmt.Errorf("--listen serves the printer and does not take --text or --image")
		}
		return serve(listen, address, os.Getenv("LT200B_PASSWORD"))
	}

	var img image.Image
	var err error
	switch {
	case text != "":
		img, err = createTextImage(text, fontSize)
	case imagePath != "":
		img, err = openImage(imagePath)
	default:
		return fmt.Errorf("you must provide either --text or --image")
	}
	if err != nil {
		return err
	}
	return printImage(address, img)
}

func openImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	return img, nil
}

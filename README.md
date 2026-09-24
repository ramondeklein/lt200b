# lt200b

The Dymo LetraTag 200B is a Bluetooth label printer that comes with an iOS and Android app. The app only allows you to print text using a couple of installed fonts. This program implements the BLE protocol and prints an image from your computer.

## Usage

Use the MAC address printed on the back of the printer. Since the printer uses pixels that are twice as high as they are wide, the aspect ratio of the image is automatically adjusted.

On macOS the printer is selected by matching that MAC to the end of the name it advertises. Text is drawn with the Go Regular font. PNG, JPEG, and GIF images are accepted.

```
go run . --address aa:bb:cc:dd:ee:ff --image hello.png
go run . --address aa:bb:cc:dd:ee:ff --text "Hello World"
```

## HTTP

`--listen` serves a page and a print endpoint. Open the listen address in a browser to print from a form.

```
go run . --address aa:bb:cc:dd:ee:ff --listen :8080
```

`POST /print` prints the request body. A PNG, JPEG, or GIF is printed as an image. Anything else is printed as text. `font-size` sets the text size (default 64).

```
curl -d 'Hello world' 'http://127.0.0.1:8080/print?font-size=48'
curl --data-binary @hello.png http://127.0.0.1:8080/print
```

Set `LT200B_PASSWORD` to require a password. The page shows a password field, and clients can send HTTP Basic or `Authorization: Bearer`.

```
LT200B_PASSWORD=secret go run . --address aa:bb:cc:dd:ee:ff --listen :8080
curl -u lt200b:secret -d 'Hello world' http://127.0.0.1:8080/print
```

The same settings can come from the environment: `LT200B_ADDRESS`, `LT200B_LISTEN`, and `LT200B_PASSWORD`.

## Docker

```
docker build -t lt200b .
docker run --rm -p 8080:8080 \
  -e LT200B_ADDRESS=aa:bb:cc:dd:ee:ff \
  -e LT200B_PASSWORD=secret \
  -v /var/run/dbus:/var/run/dbus \
  lt200b
```

`LT200B_LISTEN` defaults to `:8080`. Leave `LT200B_PASSWORD` empty to leave the page open. The container uses the host BlueZ daemon through the D-Bus socket.

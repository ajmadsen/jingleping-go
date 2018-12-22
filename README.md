# jingleping-go

Ping images to the [IPv6 christmas tree](https://jinglepings.com). I wrote this application to draw a dabbing Pikachu and a dancing Sans to the video wall.

![Screenshot of Video Board with Pikachu and Sans](doc/tree.jpg)

# Usage

```
Usage of ./jingleping-go:
  -dst-net string
    	the destination network of the ipv6 tree (default "2001:4c08:2028")
  -image string
    	the image to ping to the tree
  -rate int
    	how many times to draw the image per second (default 100)
  -workers int
    	the number of workers to use (default 1)
  -x int
    	the x offset to draw the image
  -y int
    	the y offset to draw the image
```

If the image is an animated GIF, the rate should be adjusted accordingly. By default, the application will send one ping per pixel per frame. If the rate is set to some value higher than the average frame delay, each pixel in the frame may be sent more than once, improving the fidelity of the animation.

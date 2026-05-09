package main

import (
	"fmt"
	"math"
	"os"
	"time"
)

const (
	width  = 30
	height = 20
	gravity = 0.05
)

type Ball struct {
	x, y   float64
	vx, vy float64
}

func main() {
	ball := Ball{x: 15, y: 5, vx: 0.2, vy: 0}
	score := 0
	
	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	fmt.Println("\033[2J") // Clear screen

	for {
		// Update Physics
		ball.vy += gravity
		ball.x += ball.vx
		ball.y += ball.vy

		// Wall Bouncing
		if ball.x <= 1 || ball.x >= width-1 {
			ball.vx *= -1
			score += 5
		}
		if ball.y <= 1 {
			ball.vy *= -1
			ball.y = 1
			score += 5
		}

		// Flipper Logic (Simple auto-flipper for demo, or hit detection)
		if ball.y >= height-2 {
			if ball.x > 10 && ball.x < 20 {
				ball.vy = -0.8 // Launch back up!
				ball.vx += (ball.x - 15) * 0.1 // Add some English
				score += 50
				fmt.Print("\a") // Beep!
			} else {
				// Game Over
				fmt.Printf("\033[%d;%dH", height/2, width/2-5)
				fmt.Print("\033[1;31m GAME OVER \033[0m")
				fmt.Printf("\033[%d;%dH", height/2+1, width/2-7)
				fmt.Printf("Final Score: %d", score)
				fmt.Printf("\033[%d;0H", height+2)
				return
			}
		}

		// Drawing
		drawBoard(ball, score)
		time.Sleep(50 * time.Millisecond)
	}
}

func drawBoard(ball Ball, score int) {
	// Move cursor to top-left
	fmt.Print("\033[H")
	
	fmt.Println("\033[1;36m🌴 MIAMI PINBALL 🌴\033[0m")
	fmt.Printf("Score: \033[1;33m%d\033[0m\n", score)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if y == 0 || y == height-1 || x == 0 || x == width-1 {
				fmt.Print("\033[1;34m#\033[0m") // Blue walls
			} else if int(ball.x) == x && int(ball.y) == y {
				fmt.Print("\033[1;37mO\033[0m") // White ball
			} else if y == height-2 && x > 10 && x < 20 {
				fmt.Print("\033[1;35m=\033[0m") // Pink flipper
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
}

//go:build !windows

package main

// WindowsToast is a no-op on non-Windows platforms.
func WindowsToast(spot FlexSpot) {}

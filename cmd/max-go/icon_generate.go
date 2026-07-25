// icon_generate.go : defines the generator for the Windows icon resource.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Package main builds the M.A.X. game executable.
// The checked in .syso supports normal builds. Run go generate only to rebuild the icon resource.
//
//go:generate windres --target=pe-x86-64 -i max_icon.rc -o max_icon_windows_amd64.syso
package main

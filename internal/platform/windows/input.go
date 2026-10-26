package winplat

// Win32 INPUT layouts for SendInput. They are plain data (no syscalls) so
// their sizes can be checked by tests on any GOOS with the same word size.
//
// INPUT is { DWORD type; union { MOUSEINPUT; KEYBDINPUT; HARDWAREINPUT } }.
// MOUSEINPUT is the largest member, so mouseINPUT has the full INPUT size;
// keybdINPUT pads KEYBDINPUT up to the same size (8 bytes on both 386 and
// amd64/arm64). Expected sizes: 40 bytes on 64-bit, 28 bytes on 32-bit.

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfKeyUp   = 0x0002
	keyeventfUnicode = 0x0004

	mouseeventfMove        = 0x0001
	mouseeventfVirtualDesk = 0x4000
	mouseeventfAbsolute    = 0x8000

	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12 // ALT
	vkLWin    = 0x5B
	vkRWin    = 0x5C
)

type mouseInput struct {
	Dx        int32
	Dy        int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type mouseINPUT struct {
	Type uint32
	Mi   mouseInput
}

type keybdINPUT struct {
	Type uint32
	Ki   keybdInput
	_    [8]byte
}

// unicodeKeys returns a KEYEVENTF_UNICODE key down/up INPUT pair per UTF-16
// code unit (a rune outside the BMP is a surrogate pair: two pairs).
func unicodeKeys(units []uint16) []keybdINPUT {
	out := make([]keybdINPUT, 0, 2*len(units))
	for _, u := range units {
		out = append(out,
			keybdINPUT{Type: inputKeyboard, Ki: keybdInput{Scan: u, Flags: keyeventfUnicode}},
			keybdINPUT{Type: inputKeyboard, Ki: keybdInput{Scan: u, Flags: keyeventfUnicode | keyeventfKeyUp}},
		)
	}
	return out
}

// ctrlChord returns Ctrl+vk as virtual-key INPUTs: Ctrl down, vk down, vk up,
// Ctrl up. vk is a Win32 virtual-key code ('S' is VK_S).
func ctrlChord(vk uint16) []keybdINPUT {
	return []keybdINPUT{
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkControl}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: vk}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: vk, Flags: keyeventfKeyUp}},
		{Type: inputKeyboard, Ki: keybdInput{Vk: vkControl, Flags: keyeventfKeyUp}},
	}
}

package coverage

import (
	"testing"
)

func TestAnalyzeVCL_SimpleSub(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    set req.http.foo = "bar";
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	if len(fb.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(fb.Blocks))
	}

	block := fb.Blocks[0]
	if block.Type != BlockTypeSub {
		t.Errorf("expected type 'sub', got %q", block.Type)
	}
	if block.Name != "vcl_recv" {
		t.Errorf("expected name 'vcl_recv', got %q", block.Name)
	}
	if block.HeaderLine != 3 {
		t.Errorf("expected header line 3, got %d", block.HeaderLine)
	}
	// The opening brace is on the same line as "sub vcl_recv {"
	if block.OpenBrace != 3 {
		t.Errorf("expected open brace line 3, got %d", block.OpenBrace)
	}
	if block.CloseBrace != 6 {
		t.Errorf("expected close brace line 6, got %d", block.CloseBrace)
	}
}

func TestAnalyzeVCL_MultipleSubs(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    return (hash);
}

sub vcl_backend_response {
    set beresp.ttl = 1h;
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	if len(fb.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(fb.Blocks))
	}

	if fb.Blocks[0].Name != "vcl_recv" {
		t.Errorf("expected first block name 'vcl_recv', got %q", fb.Blocks[0].Name)
	}
	if fb.Blocks[1].Name != "vcl_backend_response" {
		t.Errorf("expected second block name 'vcl_backend_response', got %q", fb.Blocks[1].Name)
	}
}

func TestAnalyzeVCL_IfStatement(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	if len(fb.Blocks) != 1 {
		t.Fatalf("expected 1 top-level block, got %d", len(fb.Blocks))
	}

	subBlock := fb.Blocks[0]
	if len(subBlock.Children) != 1 {
		t.Fatalf("expected 1 child block (if), got %d", len(subBlock.Children))
	}

	ifBlock := subBlock.Children[0]
	if ifBlock.Type != BlockTypeIf {
		t.Errorf("expected type 'if', got %q", ifBlock.Type)
	}
	if ifBlock.HeaderLine != 4 {
		t.Errorf("expected if header line 4, got %d", ifBlock.HeaderLine)
	}
	if ifBlock.OpenBrace != 4 {
		t.Errorf("expected if open brace line 4, got %d", ifBlock.OpenBrace)
	}
	if ifBlock.CloseBrace != 6 {
		t.Errorf("expected if close brace line 6, got %d", ifBlock.CloseBrace)
	}
}

func TestAnalyzeVCL_IfElseChain(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    } else if (req.url ~ "^/static") {
        return (hash);
    } else {
        return (synth(404));
    }
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	subBlock := fb.Blocks[0]
	if len(subBlock.Children) != 3 {
		t.Fatalf("expected 3 child blocks (if, elseif, else), got %d", len(subBlock.Children))
	}

	// Check if block
	if subBlock.Children[0].Type != BlockTypeIf {
		t.Errorf("expected first child type 'if', got %q", subBlock.Children[0].Type)
	}

	// Check elseif block
	if subBlock.Children[1].Type != BlockTypeElseIf {
		t.Errorf("expected second child type 'elseif', got %q", subBlock.Children[1].Type)
	}

	// Check else block
	if subBlock.Children[2].Type != BlockTypeElse {
		t.Errorf("expected third child type 'else', got %q", subBlock.Children[2].Type)
	}
}

func TestAnalyzeVCL_NestedIf(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        if (req.method == "POST") {
            return (pass);
        }
    }
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	subBlock := fb.Blocks[0]
	if len(subBlock.Children) != 1 {
		t.Fatalf("expected 1 child block, got %d", len(subBlock.Children))
	}

	outerIf := subBlock.Children[0]
	if len(outerIf.Children) != 1 {
		t.Fatalf("expected 1 nested block, got %d", len(outerIf.Children))
	}

	innerIf := outerIf.Children[0]
	if innerIf.Type != BlockTypeIf {
		t.Errorf("expected nested block type 'if', got %q", innerIf.Type)
	}
}

func TestFileBlocks_FindBlockAtLine(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Line 5 is inside the if block
	block := fb.FindBlockAtLine(5)
	if block == nil {
		t.Fatal("expected to find a block at line 5")
	}
	if block.Type != BlockTypeIf {
		t.Errorf("expected block type 'if' at line 5, got %q", block.Type)
	}

	// Line 7 is inside the sub but not in the if
	block = fb.FindBlockAtLine(7)
	if block == nil {
		t.Fatal("expected to find a block at line 7")
	}
	if block.Type != BlockTypeSub {
		t.Errorf("expected block type 'sub' at line 7, got %q", block.Type)
	}

	// Line 1 is outside any block
	block = fb.FindBlockAtLine(1)
	if block != nil {
		t.Errorf("expected no block at line 1, got %v", block)
	}
}

func TestFileBlocks_GetLineStatus(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Mark the sub as entered, but not the if
	fb.Blocks[0].Entered = true
	fb.Blocks[0].Children[0].Entered = false

	status := fb.GetLineStatus()

	// Line 3 (sub open brace) should be entered (true)
	if !status[3] {
		t.Errorf("expected line 3 to be entered")
	}

	// Line 7 (return hash inside sub but outside if) should be entered
	if !status[7] {
		t.Errorf("expected line 7 to be entered")
	}

	// Line 5 (inside if block) should NOT be entered (child overrides parent)
	if status[5] {
		t.Errorf("expected line 5 to NOT be entered (if block not entered)")
	}

	// Line 1 should not be in the map (outside any block)
	if _, exists := status[1]; exists {
		t.Errorf("expected line 1 to not be in status map")
	}
}

package coverage

import (
	"testing"
)

func TestMatchTracesToBlocks_Simple(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    return (hash);
}
`
	// Block structure:
	// vcl_recv: lines 3-5 (statement at line 4)
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Trace at line 4 (the return statement inside vcl_recv)
	MatchTracesToBlocks(fb, []int{4})

	if !fb.Blocks[0].Entered {
		t.Error("expected vcl_recv block to be marked as entered")
	}
}

func TestMatchTracesToBlocks_IfBlock(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	// Block structure:
	// vcl_recv: lines 3-8
	//   if: lines 4-6 (statement at line 5)
	// Line 7 is "return (hash)" inside sub but outside if
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Trace at line 5 (inside if) and line 7 (inside sub, outside if)
	MatchTracesToBlocks(fb, []int{5, 7})

	if !fb.Blocks[0].Entered {
		t.Error("expected vcl_recv block to be marked as entered")
	}
	if !fb.Blocks[0].Children[0].Entered {
		t.Error("expected if block to be marked as entered")
	}
}

func TestMatchTracesToBlocks_SubEnteredIfNot(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	// Block structure:
	// vcl_recv: lines 3-8
	//   if: lines 4-6
	// Line 7 is inside sub but outside if
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Only trace line 7 (inside sub, outside if)
	MatchTracesToBlocks(fb, []int{7})

	if !fb.Blocks[0].Entered {
		t.Error("expected vcl_recv block to be marked as entered")
	}
	if fb.Blocks[0].Children[0].Entered {
		t.Error("expected if block to NOT be marked as entered")
	}
}

func TestMatchTracesToBlocks_IfElseChain(t *testing.T) {
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

	// Block structure:
	// sub vcl_recv: lines 3-11
	//   if: lines 4-6 (statement at line 5)
	//   elseif: lines 6-8 (statement at line 7)
	//   else: lines 8-10 (statement at line 9)
	//
	// VCL_trace fires at STATEMENTS inside blocks, not at braces.
	// If only the elseif branch was taken, we'd see traces for:
	// - First statement in sub (but there's none before the if)
	// - Statement inside elseif (line 7)
	MatchTracesToBlocks(fb, []int{7})

	sub := fb.Blocks[0]
	if !sub.Entered {
		t.Error("expected sub to be entered (has entered child)")
	}

	ifBlock := sub.Children[0]
	if ifBlock.Entered {
		t.Error("expected if block to NOT be entered")
	}

	elseifBlock := sub.Children[1]
	if !elseifBlock.Entered {
		t.Error("expected elseif block to be entered")
	}

	elseBlock := sub.Children[2]
	if elseBlock.Entered {
		t.Error("expected else block to NOT be entered")
	}
}

func TestMatchTracesToBlocks_NoTraces(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// No traces
	MatchTracesToBlocks(fb, []int{})

	if fb.Blocks[0].Entered {
		t.Error("expected vcl_recv block to NOT be marked as entered")
	}
}

func TestMatchTracesToBlocksWithTolerance(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    return (hash);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Trace line 4 (one off from opening brace at line 3)
	MatchTracesToBlocksWithTolerance(fb, []int{4}, 1)

	if !fb.Blocks[0].Entered {
		t.Error("expected vcl_recv block to be marked as entered (with tolerance)")
	}
}

func TestGetEnteredBlocks(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}

sub vcl_synth {
    return (deliver);
}
`
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Mark vcl_recv and its if block as entered, vcl_synth not entered
	fb.Blocks[0].Entered = true
	fb.Blocks[0].Children[0].Entered = true
	fb.Blocks[1].Entered = false

	entered := GetEnteredBlocks(fb)
	if len(entered) != 2 {
		t.Errorf("expected 2 entered blocks, got %d", len(entered))
	}

	notEntered := GetNotEnteredBlocks(fb)
	if len(notEntered) != 1 {
		t.Errorf("expected 1 not-entered block, got %d", len(notEntered))
	}
	if notEntered[0].Name != "vcl_synth" {
		t.Errorf("expected not-entered block to be vcl_synth, got %s", notEntered[0].Name)
	}
}

func TestLineStatusAfterMatching(t *testing.T) {
	vcl := `vcl 4.1;

sub vcl_recv {
    if (req.url ~ "^/api") {
        return (pass);
    }
    return (hash);
}
`
	// Block structure:
	// vcl_recv: lines 3-8
	//   if: lines 4-6
	// Line 7 is inside sub but outside if
	fb, err := AnalyzeVCL(vcl, "/test.vcl")
	if err != nil {
		t.Fatalf("AnalyzeVCL failed: %v", err)
	}

	// Sub entered (trace at line 7), if NOT entered
	MatchTracesToBlocks(fb, []int{7})

	status := fb.GetLineStatus()

	// Line 3 (sub open brace) - part of entered sub block
	if !status[3] {
		t.Error("expected line 3 to be entered")
	}

	// Line 7 (return hash, inside sub but outside if) - entered
	if !status[7] {
		t.Error("expected line 7 to be entered")
	}

	// Line 5 (inside if block) - NOT entered (child overrides parent)
	if status[5] {
		t.Error("expected line 5 to NOT be entered")
	}
}

// Package declaw provides a Go client SDK for the Declaw sandboxing platform.
//
// Declaw is a security-first sandboxing platform for AI agents. This SDK
// allows you to create, manage, and interact with sandboxed environments
// including running commands, managing files, and configuring security policies.
//
// Basic usage:
//
//	ctx := context.Background()
//	sandbox, err := declaw.Create(ctx,
//	    declaw.WithTemplate("python"),
//	    declaw.WithTimeout(300),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.Kill(ctx)
//
//	result, err := sandbox.Commands.Run(ctx, "echo hello")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Stdout)
package declaw

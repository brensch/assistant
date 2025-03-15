## Defining Command Request Structs

To leverage the automatic mapping of interaction options, define your command request structs with exported fields and use the `discord` struct tag to provide additional metadata. Supported tags include:

- **`optional`**:  
  Marks the field as non-required. If omitted in the interaction, it won’t trigger an error.

- **`description`**:  
  Overrides the default description generated for the command option.

- **`choices`**:  
  Supplies a semicolon‑separated list of predefined choices in the format `value|label`. This will be used to populate a dropdown list in Discord.

- **`default`**:  
  Specifies a default value that should be set if the field is not provided during the interaction.

**Example:**

```go
type GreetRequest struct {
	Username string `discord:"description:The username to greet,default:Guest"`
	Times    int    `discord:"optional,description:Number of greetings"`
	Color    string `discord:"optional,description:Favorite color,choices:red|Red;blue|Blue;green|Green,default:blue"`
}
```
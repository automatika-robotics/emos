# Tool Calling

In the previous [recipe](goto-navigation.md) we created a Go-to-X component using basic text manipulation on LLM output. However, for models that have been specifically trained for tool calling, one can get better results for structured outputs by invoking tool calling. At the same time tool calling can be useful to generate responses which require intermediate use of tools by the LLM before providing a final answer. In this recipe we will utilize tool calling for the former utility of getting a better structured output from the LLM, by reimplementing the Go-to-X component.

## Register a tool (function) to be called by the LLM

To utilize tool calling we will change our strategy of doing pre-processing to LLM text output, and instead ask the LLM to provide structured input to a function (tool). The output of this function will then be sent for publishing to the output topic. Lets see what this will look like in the following code snippets.

First we will modify the component level prompt for our LLM.

```python
# set a component prompt
goto.set_component_prompt(
    template="""What are the position coordinates in the given metadata?"""
)
```

Next we will replace our pre-processing function, with a much simpler function that takes in a list and provides a numpy array. The LLM will be expected to call this function with the appropriate output. This strategy generally works better than getting text input from LLM and trying to parse it with an arbitrary function. To register the function as a tool, we will also need to create its description in a format that is explanatory for the LLM. This format has been specified by the _Ollama_ client.

```{caution}
Tool calling is currently available only when components utilize the OllamaClient.
```

```{seealso}
To see a list of models that work for tool calling using the OllamaClient, check [here](https://ollama.com/search?c=tools)
```

```python
# pre-process the output before publishing to a topic of msg_type PoseStamped
def get_coordinates(position: list[float]) -> np.ndarray:
    """Get position coordinates"""
    return np.array(position, dtype=float)


function_description = {
    "type": "function",
    "function": {
        "name": "get_coordinates",
        "description": "Get position coordinates",
        "parameters": {
            "type": "object",
            "properties": {
                "position": {
                    "type": "list[float]",
                    "description": "The position coordinates in x, y and z",
                }
            },
        },
        "required": ["position"],
    },
}

# add the pre-processing function to the goal_point output topic
goto.register_tool(
    tool=get_coordinates,
    tool_description=function_description,
    send_tool_response_to_model=False,
)
```

In the code above, the flag _send_tool_response_to_model_ has been set to False. This means that the function output will be sent directly for publication, since our usage of the tool in this recipe is limited to forcing the model to provide a structured output. If this flag was set to True, the output of the tool (function) will be sent back to the model to produce the final output, which will then be published. This latter usage is employed when a tool like a calculator, browser or code interpreter can be provided to the model for generating better answers.

## Launching the Components

And as before, we will launch our Go-to-X component.

```python
from agents.ros import Launcher

# Launch the component
launcher = Launcher()
launcher.add_pkg(components=[goto])
launcher.bringup()
```

The complete code for this recipe is given below:

```{code-block} python
:caption: Go-to-X Component with Tool Calling
:linenos:
import numpy as np
from agents.components import LLM
from agents.models import OllamaModel
from agents.vectordbs import ChromaDB
from agents.config import LLMConfig
from agents.clients import ChromaClient, OllamaClient
from agents.ros import Launcher, Topic

# Start a Llama3.2 based llm component using ollama client
llama = OllamaModel(name="llama", checkpoint="llama3.2:3b")
llama_client = OllamaClient(llama)

# Initialize a vector DB that will store our routes
chroma = ChromaDB()
chroma_client = ChromaClient(db=chroma)

# Define LLM input and output topics including goal_point topic of type PoseStamped
goto_in = Topic(name="goto_in", msg_type="String")
goal_point = Topic(name="goal_point", msg_type="PoseStamped")

config = LLMConfig(
    enable_rag=True,
    collection_name="map",
    distance_func="l2",
    n_results=1,
    add_metadata=True,
)

# initialize the component
goto = LLM(
    inputs=[goto_in],
    outputs=[goal_point],
    model_client=llama_client,
    db_client=chroma_client,  # check the previous example where we setup this database client
    trigger=goto_in,
    config=config,
    component_name="go_to_x",
)

# set a component prompt
goto.set_component_prompt(
    template="""What are the position coordinates in the given metadata?"""
)


# pre-process the output before publishing to a topic of msg_type PoseStamped
def get_coordinates(position: list[float]) -> np.ndarray:
    """Get position coordinates"""
    return np.array(position, dtype=float)


function_description = {
    "type": "function",
    "function": {
        "name": "get_coordinates",
        "description": "Get position coordinates",
        "parameters": {
            "type": "object",
            "properties": {
                "position": {
                    "type": "list[float]",
                    "description": "The position coordinates in x, y and z",
                }
            },
        },
        "required": ["position"],
    },
}

# add the pre-processing function to the goal_point output topic
goto.register_tool(
    tool=get_coordinates,
    tool_description=function_description,
    send_tool_response_to_model=False,
)

# Launch the component
launcher = Launcher()
launcher.add_pkg(components=[goto])
launcher.bringup()
```

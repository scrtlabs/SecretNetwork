const testInput = {
    events: [
        {
            type: "yes",
            attributes: [
                {
                    key: {
                        "0": 115,
                        "1": 101,
                        "2": 110,
                        "3": 100,
                        "4": 101,
                        "5": 114
                    },
                    value: {
                        "0": 115,
                        "1": 101,
                        "2": 99,
                        "3": 114,
                    },
                    index: true
                }
            ]
        }
    ],
    "other": "value"
}

interface BytesObj {
    [key: string]: number
}

const bytesToKv = (input: BytesObj) => {
    let output = "";
    for (const v of Object.values(input)) {
        output += String.fromCharCode(v);
    }

    return output;
}

const objToKv = (input) => {
    // console.log("got object:", input);
    const output = {};
    const key = bytesToKv(input.key);
    output[key] = bytesToKv(input.value);
    return output;
}

export const cleanBytes = (tx) => {
    // console.log("input:", JSON.stringify(testInput, null, 2), "\n\n");

    const events = tx.events.map(e => {
            return {
                ...e,
                attributes: e.attributes.map(i => objToKv(i))
            };
        }
    )

    const output = {
        ...tx,
        events,
        txBytes: undefined,
    };
    // console.log("output:", JSON.stringify(output, null, 2));
    return output;
}
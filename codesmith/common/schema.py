from schema import And
from box import Box
import json


def not_empty(x):
    return bool(x)


encoded_bool = And(str, json.loads, bool)

non_empty_string = And(str, not_empty)


def box(properties, *, schema):
    return Box(schema.validate(properties), camel_killer_box=True, default_box=True, default_box_attr=None)

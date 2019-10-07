from schema import And, Schema, Use
from box import Box
import json


def tolerant_schema(s):
    return Schema(s, ignore_extra_keys=True)


def not_empty(x):
    return bool(x)


encoded_bool = And(str, Use(json.loads), bool)

encoded_int = And(str, Use(json.loads), int)

non_empty_string = And(str, not_empty)


def box(properties, *, schema):
    return Box(schema.validate(properties), camel_killer_box=True, default_box=True, default_box_attr=None)

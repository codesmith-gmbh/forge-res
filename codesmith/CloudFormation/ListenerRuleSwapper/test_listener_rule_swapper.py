import unittest
import codesmith.CloudFormation.ListenerRuleSwapper.listener_rule_swapper as lrs

RULE_ARN = 'arn:aws:elasticloadbalancing:us-west-2:123456789012:listener-rule/app/my-load-balancer/50dc6c495c0c9188/f2f7dc8efc522ab2/9683b2d02a6cabee'
RULES = {
    'Rules': [
        {
            'Actions': [
                {
                    'TargetGroupArn': 'arn:aws:elasticloadbalancing:us-west-2:123456789012:targetgroup/my-targets/73e2d6bc24d8a067',
                    'Type': 'forward',
                },
            ],
            'Conditions': [
                {
                    'Field': 'path-pattern',
                    'Values': [
                        '/img/*',
                    ],
                },
            ],
            'IsDefault': False,
            'Priority': '10',
            'RuleArn': RULE_ARN,
        },
    ]
}


class TestListenerRuleSwapper(unittest.TestCase):
    def test_rule_priorities(self):
        priorities = lrs.rule_priorities(RULES)
        self.assertEqual(1, len(priorities))
        self.assertEqual(10, priorities[RULE_ARN])


if __name__ == '__main__':
    unittest.main()

import unittest
import aws.CodeCommit.PipelineTrigger.pipline_trigger as pt
import json
import io
import zipfile

COMMIT_REFERENCE = {
    "commit": "5c4ef1049f1d27deadbeeff313e0730018be182b",
    "ref": "refs/heads/master"
}
TAG_REFERENCE = {
    "commit": "5c4ef1049f1d27deadbeeff313e0730018be182b",
    "ref": "refs/tags/v1.1.0"
}


class PipelineTriggerTest(unittest.TestCase):

    def test_extract_repository_name(self):
        self.assertEqual('my-repo', pt.extract_repository_name('arn:aws:codecommit:eu-west-1:123456789012:my-repo'))

        self.assertEqual('', pt.extract_repository_name(''))
        self.assertEqual('anything', pt.extract_repository_name('anything'))

    def test_is_commit(self):
        self.assertTrue(pt.is_commit(COMMIT_REFERENCE))
        self.assertFalse(pt.is_commit(TAG_REFERENCE))

    def test_is_tag(self):
        self.assertTrue(pt.is_tag(TAG_REFERENCE))
        self.assertFalse(pt.is_tag(COMMIT_REFERENCE))

    def test_extract_tag(self):
        self.assertEqual('v1.1.0', pt.extract_tag(TAG_REFERENCE))

    def test_event(self):
        with open('code_commit_event.json') as f:
            event = json.load(f)
        pipeline_trigger = pt.derive_trigger(event['Records'][0])

        self.assertEqual('eu-west-1', pipeline_trigger.aws_region)
        self.assertEqual('my-repo', pipeline_trigger.repository)
        self.assertEqual('git checkout 5c4ef1049f1d27deadbeeff313e0730018be182b', pipeline_trigger.checkout_command)

        buf = io.BytesIO(pipeline_trigger.generate_zip_file())
        with zipfile.ZipFile(buf) as zf:
            given_files = [file.filename for file in zf.filelist]
            expected_files = ['buildspec.yaml', 'chechkout.sh']
            self.assertEqual(expected_files, given_files)

        given_checkout_text = pipeline_trigger.generate_files()['chechkout.sh']
        expected_checkout_text = '''#!/bin/bash

git config --global credential.helper '!aws codecommit credential-helper $@'
git config --global credential.UseHttpPath true

git clone --shallow-submodules https://git-codecommit.eu-west-1.amazonaws.com/v1/repos/my-repo repo
cd repo
git checkout 5c4ef1049f1d27deadbeeff313e0730018be182b
cd
'''
        self.assertEqual(expected_checkout_text, given_checkout_text)


if __name__ == '__main__':
    unittest.main()

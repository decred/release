#!/bin/sh

find dist -name '*-manifest.txt' | perl -MDigest::SHA -lne \
	'print Digest::SHA->new(256)->addfile($_)->hexdigest, "  file://$_"'
find thirdparty -type f | perl -MDigest::SHA -lne '\
	open (my $f, "<", $_);
	chomp(my $url = <$f>);
	local $/;
	my $contents = <$f>;
	my $sum = Digest::SHA->new(256)->add($contents)->hexdigest;
	print "$sum  $url";'

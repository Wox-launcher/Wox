using System;
using System.Collections;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.IO;
using System.Linq;
using System.Security;

// This code is public domain - from the StackOverflow post at
// <https://stackoverflow.com/questions/13130052/directoryinfo-enumeratefiles-causes-unauthorizedaccessexception-and-other>
// Provided by Matthew Brubaker.  I do not own this code in any way; I am merely putting this here to facilitate
// access by the public/improvements.

// CHANGELOG
//
// 26 Jan 2019 (Brian Hart): Added two overloads of a static Search() method so that you do not have to call
// the constructor directly.  Helpful for cleaner syntax in loops.
// 29 Jan 2019 (Brian Hart) Did a CodeMaid code cleanup on the file (formatted and sorted usings) and commented out
// all the calls to ILog, to remove the dependency on log4net, which might not be something that everyone who wants to 
// utilize this class might also utilize.
//
// Source: https://github.com/astrohart/FileSystemEnumerable
//

/// <summary>
/// Enumerates the files and folders in the specified directory, yielding results as they come, and 
/// skipping files and folders to which the operating system denies us access.	
/// </summary>
/// <remarks>
/// The functionality in this class is known to work, so it has been marked with the [ExcludeFromCodeCoverage]
/// attribute.
/// </remarks>
[ExcludeFromCodeCoverage]
public class FileSystemEnumerable : IEnumerable<FileSystemInfo>
{
    private readonly DirectoryInfo _root;
    private readonly IList<string> _patterns;
    private readonly SearchOption _option;

    public static IEnumerable<FileSystemInfo> Search(DirectoryInfo root, string pattern = "*",
        SearchOption option = SearchOption.AllDirectories)
    {
        if (!root.Exists)
            throw new DirectoryNotFoundException($"The folder '{root.FullName}' could not be located.");

        /* If the search pattern string is blank, then default to the wildcard (*) pattern. */
        if (string.IsNullOrWhiteSpace(pattern))
            pattern = "*";

        return new FileSystemEnumerable(root, pattern, option);
    }

    public static IEnumerable<FileSystemInfo> Search(string root, string pattern = "*",
        SearchOption option = SearchOption.AllDirectories)
    {
        if (!Directory.Exists(root))
            throw new DirectoryNotFoundException($"The folder '{root}' could not be located.");

        /* If the search pattern string is blank, then default to the wildcard (*) pattern. */
        if (string.IsNullOrWhiteSpace(pattern))
            pattern = "*";

        var rootDirectoryInfo = new DirectoryInfo(root);
        return new FileSystemEnumerable(rootDirectoryInfo, pattern, option);
    }

    public FileSystemEnumerable(DirectoryInfo root, string pattern, SearchOption option)
    {
        _root = root;
        _patterns = new List<string> { pattern };
        _option = option;
    }

    public FileSystemEnumerable(DirectoryInfo root, IList<string> patterns, SearchOption option)
    {
        _root = root;
        _patterns = patterns;
        _option = option;
    }

    public IEnumerator<FileSystemInfo> GetEnumerator()
    {
        if (_root == null || !_root.Exists) yield break;

        IEnumerable<FileSystemInfo> matches = new List<FileSystemInfo>();
        try
        {
            //_logger.DebugFormat("Attempting to enumerate '{0}'", _root.FullName);
            matches = _patterns.Aggregate(matches, (current, pattern) =>
                current.Concat(_root.EnumerateDirectories(pattern, SearchOption.TopDirectoryOnly))
                        .Concat(_root.EnumerateFiles(pattern, SearchOption.TopDirectoryOnly)));
        }
        catch (UnauthorizedAccessException)
        {
            //_logger.WarnFormat("Unable to access '{0}'. Skipping...", _root.FullName);
            yield break;
        }
        catch (SecurityException)
        {
            //_logger.WarnFormat("Unable to access '{0}'. Skipping...", _root.FullName);
            yield break;
        }
        catch (PathTooLongException)
        {
            //_logger.Warn(string.Format(@"Could not process path '{0}\{1}'.", _root.Parent.FullName, _root.Name), ptle);
            yield break;
        }
        catch (System.IO.IOException)
        {
            // "The symbolic link cannot be followed because its type is disabled."
            // "The specified network name is no longer available."
            //_logger.Warn(string.Format(@"Could not process path (check SymlinkEvaluation rules)'{0}\{1}'.", _root.Parent.FullName, _root.Name), e);
            yield break;
        }


        //_logger.DebugFormat("Returning all objects that match the pattern(s) '{0}'", string.Join(",", _patterns));
        foreach (var file in matches)
        {
            yield return file;
        }

        if (_option != SearchOption.AllDirectories)
            yield break;

        //_logger.DebugFormat("Enumerating all child directories.");
        foreach (var dir in _root.EnumerateDirectories("*", SearchOption.TopDirectoryOnly))
        {
            //_logger.DebugFormat("Enumerating '{0}'", dir.FullName);
            var fileSystemInfos = new FileSystemEnumerable(dir, _patterns, _option);
            foreach (var match in fileSystemInfos)
            {
                yield return match;
            }
        }
    }

    IEnumerator IEnumerable.GetEnumerator()
    {
        return GetEnumerator();
    }
}